// Package api exposes the REST surface: issue a document, verify an uploaded
// PDF against the chain, and list what a citizen holds.
package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"credreg/backend/internal/auth"
	"credreg/backend/internal/chain"
	"credreg/backend/internal/docid"
	"credreg/backend/internal/pdfdoc"
	"credreg/backend/internal/rbac"
	"credreg/backend/internal/store"
)

const maxUpload = 20 << 20 // 20 MB, plenty for a demo certificate

// reader is the read side of the registry. Splitting it out keeps the verify
// state machine testable without an Anvil node behind it.
type reader interface {
	Verify(ctx context.Context, docHash [32]byte) (chain.Record, error)
	CurrentHashOf(ctx context.Context, docID string) ([32]byte, bool, error)
}

type registry interface {
	reader
	Issue(context.Context, chain.Department, [32]byte, string, uint8) (string, error)
	Supersede(context.Context, chain.Department, [32]byte, [32]byte, string, uint8) (string, error)
	Revoke(context.Context, chain.Department, [32]byte, uint8) (string, error)
}

type Server struct {
	chain           registry
	reader          reader
	store           *store.Store
	contractAddress string
	publicWebURL    string
	notifications   notifier
}

func New(c *chain.Client, s *store.Store, verifier auth.TokenVerifier, publicWebURL string) http.Handler {
	return newHandlerConfigured(c, s, verifier, c.Address.Hex(), publicWebURL, newDevelopmentNotifier())
}

func newHandler(c registry, s *store.Store, verifier auth.TokenVerifier, contractAddress string) http.Handler {
	return newHandlerConfigured(c, s, verifier, contractAddress, "http://127.0.0.1:5173", newDevelopmentNotifier())
}

func newHandlerConfigured(c registry, s *store.Store, verifier auth.TokenVerifier, contractAddress, publicWebURL string, notifications notifier) http.Handler {

	srv := &Server{chain: c, reader: c, store: s, contractAddress: contractAddress, publicWebURL: strings.TrimRight(publicWebURL, "/"), notifications: notifications}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", srv.health)
	mux.HandleFunc("GET /departments", srv.departments)

	authenticated := func(next http.Handler) http.Handler {
		return auth.Middleware(verifier)(rbac.LoadProfile(s)(next))
	}
	identityAuthenticated := func(next http.Handler) http.Handler {
		return auth.Middleware(verifier)(next)
	}
	protected := func(permission rbac.Permission, handler http.HandlerFunc) http.Handler {
		return authenticated(rbac.Require(permission)(handler))
	}
	requestProtected := func(handler http.HandlerFunc) http.Handler {
		return authenticated(srv.auditDeniedOnly(actionRequestCreated, rbac.Require(rbac.PermissionRequestVerification)(handler)))
	}
	audited := func(permission rbac.Permission, action string, handler http.HandlerFunc) http.Handler {
		return authenticated(srv.audit(action, rbac.Require(permission)(handler)))
	}

	mux.Handle("GET /me", authenticated(http.HandlerFunc(srv.me)))
	mux.Handle("GET /account", identityAuthenticated(http.HandlerFunc(srv.account)))
	mux.Handle("GET /citizen/credentials", identityAuthenticated(http.HandlerFunc(srv.citizenCredentials)))
	mux.Handle("GET /citizen/documents/{hash}/download", identityAuthenticated(http.HandlerFunc(srv.citizenDownload)))
	mux.Handle("GET /citizen/verification-requests", identityAuthenticated(http.HandlerFunc(srv.citizenVerificationRequests)))
	mux.Handle("POST /citizen/verification-requests/{id}/decision", identityAuthenticated(http.HandlerFunc(srv.decideCitizenVerificationRequest)))
	mux.Handle("GET /citizens", protected(rbac.PermissionViewDept, srv.citizens))
	mux.Handle("POST /issue", audited(rbac.PermissionIssue, actionIssue, srv.issue))
	mux.Handle("POST /verify", audited(rbac.PermissionVerify, actionVerifyFile, srv.verify))
	mux.Handle("GET /verify/{docId}", audited(rbac.PermissionVerify, actionVerifyID, srv.verifyByID))
	mux.Handle("POST /revoke", audited(rbac.PermissionRevoke, actionRevoke, srv.revoke))
	mux.Handle("POST /supersede", audited(rbac.PermissionSupersede, actionSupersede, srv.supersede))
	mux.Handle("GET /credentials/{citizen}", protected(rbac.PermissionViewDept, srv.credentials))
	mux.Handle("GET /documents/{hash}/download", protected(rbac.PermissionViewDept, srv.download))
	mux.Handle("GET /audit-events", authenticated(http.HandlerFunc(srv.auditEventsAccess)))
	mux.Handle("GET /monitoring/overview", protected(rbac.PermissionMonitorAll, srv.monitoringOverview))
	mux.Handle("GET /citizen-accounts", protected(rbac.PermissionIssue, srv.citizenAccounts))
	mux.Handle("POST /verification-requests", requestProtected(srv.createVerificationRequest))
	mux.Handle("GET /verification-requests", requestProtected(srv.verificationRequests))
	mux.Handle("GET /verification-requests/{id}", requestProtected(srv.verificationRequest))
	mux.Handle("POST /verification-requests/{id}/complete", requestProtected(srv.completeVerificationRequest))
	mux.Handle("GET /development/notifications/{id}", requestProtected(srv.developmentNotification))
	mux.HandleFunc("GET /consent/{token}", srv.consentDetails)
	mux.HandleFunc("POST /consent/{token}/approve", srv.approveConsent)
	mux.HandleFunc("POST /consent/{token}/deny", srv.denyConsent)

	// TEMPORARY: test Supabase JWT authentication.
	mux.Handle("GET /auth-test", auth.Middleware(verifier)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := auth.UserFromContext(r.Context())
			if !ok {
				http.Error(w, "user missing from context", http.StatusInternalServerError)
				return
			}

			writeJSON(w, http.StatusOK, map[string]any{
				"id":    user.ID,
				"email": user.Email,
			})
		}),
	))

	return cors(mux)
}

// --- handlers ---------------------------------------------------------------

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":       true,
		"contract": s.contractAddress,
	})
}

func (s *Server) departments(w http.ResponseWriter, r *http.Request) {
	type departmentResponse struct {
		ID          string `json:"id"`
		Slug        string `json:"slug"`
		Name        string `json:"name"`
		DocType     uint8  `json:"docType"`
		DocTypeName string `json:"docTypeName"`
		Address     string `json:"address"`
	}
	departments, err := s.store.Departments()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := []departmentResponse{}
	for _, d := range departments {
		if !d.Active {
			continue
		}
		chainDepartment, ok := chain.DepartmentBySlug(d.ID)
		if !ok || chainDepartment.DocType != d.DocType {
			continue
		}
		out = append(out, departmentResponse{
			ID: d.ID, Slug: d.ID, Name: d.DisplayName, DocType: d.DocType,
			DocTypeName: chain.DocTypeName(d.DocType), Address: chainDepartment.Address,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	principal, ok := rbac.PrincipalFromContext(r.Context())
	if !ok {
		writeErr(w, http.StatusInternalServerError, "application profile missing from context")
		return
	}

	type departmentResponse struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		DocType     uint8  `json:"docType"`
		DocTypeName string `json:"docTypeName"`
	}
	type meResponse struct {
		ID         string              `json:"id"`
		Email      string              `json:"email"`
		Name       string              `json:"name"`
		Role       string              `json:"role"`
		Active     bool                `json:"active"`
		Department *departmentResponse `json:"department"`
	}

	response := meResponse{
		ID: principal.Profile.SupabaseUserID, Email: principal.Profile.Email,
		Name: principal.Profile.DisplayName, Role: principal.Profile.Role,
		Active: principal.Profile.Active,
	}
	if d := principal.Profile.Department; d != nil {
		response.Department = &departmentResponse{
			ID: d.ID, Name: d.DisplayName, DocType: d.DocType,
			DocTypeName: chain.DocTypeName(d.DocType),
		}
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) account(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeErr(w, http.StatusInternalServerError, "authenticated user missing from context")
		return
	}

	government, governmentErr := s.store.UserProfileByID(user.ID)
	citizen, citizenErr := s.store.CitizenAccountBySupabaseUserID(user.ID)
	governmentFound := governmentErr == nil
	citizenFound := citizenErr == nil
	if governmentErr != nil && !errors.Is(governmentErr, store.ErrNotFound) {
		writeErr(w, http.StatusInternalServerError, "could not load application account")
		return
	}
	if citizenErr != nil && !errors.Is(citizenErr, store.ErrNotFound) {
		writeErr(w, http.StatusInternalServerError, "could not load application account")
		return
	}
	if governmentFound && citizenFound {
		writeErr(w, http.StatusConflict, "identity is linked to both government and citizen accounts")
		return
	}
	if !governmentFound && !citizenFound {
		writeErr(w, http.StatusForbidden, "no application account is linked to this identity")
		return
	}

	if governmentFound {
		if !government.Active || (government.Department != nil && !government.Department.Active) {
			writeErr(w, http.StatusForbidden, "application account is inactive")
			return
		}
		var department any
		if d := government.Department; d != nil {
			department = map[string]any{
				"id": d.ID, "name": d.DisplayName, "docType": d.DocType,
				"docTypeName": chain.DocTypeName(d.DocType),
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"accountType": "GOVERNMENT",
			"id":          government.SupabaseUserID,
			"email":       government.Email,
			"governmentProfile": map[string]any{
				"id": government.SupabaseUserID, "email": government.Email,
				"name": government.DisplayName, "role": government.Role,
				"active": government.Active, "department": department,
			},
			"citizenProfile": nil,
		})
		return
	}

	if !citizen.Active {
		writeErr(w, http.StatusForbidden, "application account is inactive")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"accountType":       "CITIZEN",
		"id":                citizen.ID,
		"email":             citizen.Email,
		"governmentProfile": nil,
		"citizenProfile": map[string]any{
			"id": citizen.ID, "email": citizen.Email,
			"displayName": citizen.DisplayName, "active": citizen.Active,
		},
	})
}

func (s *Server) citizens(w http.ResponseWriter, r *http.Request) {
	_, docTypeName, ok := requestDepartment(w, r)
	if !ok {
		return
	}
	names, err := s.store.CitizensByDocType(docTypeName)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, names)
}

func (s *Server) citizenAccounts(w http.ResponseWriter, r *http.Request) {
	accounts, err := s.store.CitizenAccounts()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not load citizen accounts")
		return
	}
	out := []map[string]any{}
	for _, account := range accounts {
		out = append(out, map[string]any{"id": account.ID, "displayName": account.DisplayName, "email": maskEmail(account.Email)})
	}
	writeJSON(w, http.StatusOK, out)
}

// issue takes a multipart upload: file, dept, doc_id, doc_type, citizen. The
// compatibility dept field must match the backend-owned authenticated profile.
func (s *Server) issue(w http.ResponseWriter, r *http.Request) {
	audit := operationFromContext(r.Context())
	pdf, filename, err := readUpload(w, r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	dept, expectedDocTypeName, ok := requestDepartment(w, r)
	if !ok {
		return
	}
	if supplied := r.FormValue("dept"); supplied != "" && supplied != dept.Slug {
		writeErr(w, http.StatusForbidden, "cannot issue for another department")
		return
	}

	docTypeName := r.FormValue("doc_type")
	docType, ok := chain.DocTypeByName(docTypeName)
	if !ok {
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("unknown doc_type %q", docTypeName))
		return
	}
	if docTypeName != expectedDocTypeName || docType != dept.DocType {
		writeErr(w, http.StatusForbidden, "document type is outside the authenticated department")
		return
	}

	docID := strings.TrimSpace(r.FormValue("doc_id"))
	citizenAccountID := strings.TrimSpace(r.FormValue("citizen_account_id"))
	account, err := s.store.CitizenAccountByID(citizenAccountID)
	if err != nil || !account.Active {
		writeErr(w, http.StatusBadRequest, "a valid active citizen_account_id is required")
		return
	}
	citizen := account.DisplayName
	audit.DocID, audit.Citizen = docID, citizen
	if docID == "" {
		writeErr(w, http.StatusBadRequest, "doc_id is required")
		return
	}

	// An upload that already carries a marker has been through here before.
	// Stamping it again would leave two ids in one file.
	if existing, marked := docid.Extract(pdf); marked {
		writeErr(w, http.StatusConflict, fmt.Sprintf("this file has already been issued as %s — upload the unstamped original", existing))
		return
	}

	// Stamp first, then hash. The citizen walks away with the stamped file, so
	// the stamped bytes are the ones the chain has to know about — hashing the
	// upload as it arrived would anchor a document nobody holds.
	stamped, marked := pdfdoc.StampWithQR(pdf, docID, s.verificationURL(docID))
	sum := sha256.Sum256(stamped)
	docHash := "0x" + hex.EncodeToString(sum[:])
	audit.DocHash = docHash

	// The contract is the authority on whether this department may issue this
	// document type. If the roles disagree, this call reverts and we surface it.
	txHash, err := s.chain.Issue(r.Context(), dept, sum, docID, docType)
	if err != nil {
		audit.Outcome = "FAILURE"
		writeErr(w, http.StatusForbidden, err.Error())
		return
	}
	audit.Transaction = txHash

	doc := store.Document{
		DocHash: docHash, DocID: docID, DocType: docTypeName, Citizen: citizen,
		Issuer: dept.Address, Filename: filename, TxHash: txHash, CitizenAccountID: &account.ID,
	}
	if err := s.store.Save(doc, stamped); err != nil {
		audit.Outcome = "PARTIAL_FAILURE"
		writeErr(w, http.StatusInternalServerError, fmt.Sprintf("anchored on chain (%s) but the off-chain save failed: %v", txHash, err))
		return
	}
	audit.Result = "VALID"

	writeJSON(w, http.StatusCreated, map[string]any{
		"docHash":          docHash,
		"txHash":           txHash,
		"issuer":           dept.Name,
		"docType":          docTypeName,
		"docId":            docID,
		"citizen":          citizen,
		"citizenAccountId": account.ID,
		"downloadUrl":      downloadURL(docHash),
		"stamped":          marked,
	})
}

func (s *Server) verificationURL(docID string) string {
	return s.publicWebURL + "/verify?docId=" + url.QueryEscape(docID)
}

// zeroHash is what an unknown docId resolves to, and what we report as the
// expected hash when there is nothing to expect.
const zeroHash = "0x" + "0000000000000000000000000000000000000000000000000000000000000000"

// verify hashes an uploaded PDF and reads the docId marker embedded in it, then
// hands both to resolve. Having the id as well as the hash is what separates a
// document that was altered from one that was never issued at all.
func (s *Server) verify(w http.ResponseWriter, r *http.Request) {
	audit := operationFromContext(r.Context())
	pdf, filename, err := readUpload(w, r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	sum := sha256.Sum256(pdf)
	computed := "0x" + hex.EncodeToString(sum[:])
	audit.DocHash = computed

	resp := map[string]any{
		"filename":     filename,
		"computedHash": computed,
		"expectedHash": zeroHash,
		"docId":        "",
	}

	// A missing marker is not decisive on its own — the hash may still be on
	// record — so resolve() gets a chance either way.
	if id, ok := docid.Extract(pdf); ok {
		resp["docId"] = id
		audit.DocID = id
	}

	if err := s.resolve(r.Context(), resp["docId"].(string), computed, resp); err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	if !authorizeVerificationResponse(w, r, resp) {
		return
	}
	audit.Result, _ = resp["status"].(string)
	audit.DocID, _ = resp["docId"].(string)
	if citizen, ok := resp["citizen"].(string); ok {
		audit.Citizen = citizen
	}
	audit.Details["expectedHash"] = resp["expectedHash"]
	if resolved, ok := resp["resolvedDocId"]; ok {
		audit.Details["resolvedDocId"] = resolved
	}
	writeJSON(w, http.StatusOK, resp)
}

// verifyByID answers the same question from an id alone — what a QR scan hits
// when the verifier is holding a phone rather than the file. Without bytes to
// hash there is no computedHash, so the verdict can only be what the registry
// currently says about the document.
func (s *Server) verifyByID(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("docId")
	audit := operationFromContext(r.Context())
	audit.DocID = id
	if current, known, err := s.reader.CurrentHashOf(r.Context(), id); err == nil && known {
		hash := "0x" + hex.EncodeToString(current[:])
		if document, found, _ := s.store.ByHash(hash); found && document.CitizenAccountID != nil {
			audit.DocHash, audit.Citizen, audit.Outcome, audit.Result = hash, document.Citizen, "DENIED", "CONSENT_REQUIRED"
			writeErr(w, http.StatusConflict, "citizen consent is required; create a verification request")
			return
		}
	}

	resp := map[string]any{
		"docId":        id,
		"expectedHash": zeroHash,
	}
	if err := s.resolve(r.Context(), id, "", resp); err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	if !authorizeVerificationResponse(w, r, resp) {
		return
	}
	audit.Result, _ = resp["status"].(string)
	if expected, ok := resp["expectedHash"].(string); ok && expected != zeroHash {
		audit.DocHash = expected
	}
	if citizen, ok := resp["citizen"].(string); ok {
		audit.Citizen = citizen
	}
	if resolved, ok := resp["resolvedDocId"]; ok {
		audit.Details["resolvedDocId"] = resolved
	}
	writeJSON(w, http.StatusOK, resp)
}

// resolve fills in the verdict.
//
// The hash comes first: a hash the registry knows is a document it really issued,
// whatever its standing today. Only when the bytes are unrecognised do we fall
// back to the embedded id, and a known id at that point means the file was
// altered. Looking the id up first would be wrong — after a supersede the id
// points at the replacement, so the untouched original would read as TAMPERED.
//
// When computed is empty the caller had no file (the QR path), so there is
// nothing to recognise and the id is all we have.
func (s *Server) resolve(ctx context.Context, id, computed string, resp map[string]any) error {
	if computed != "" {
		raw, err := parseHash(computed)
		if err != nil {
			return err
		}
		rec, err := s.reader.Verify(ctx, raw)
		if err != nil {
			return err
		}
		if rec.Found {
			// These exact bytes are on record. The document is genuine; its status
			// says whether it is still the one to rely on.
			resp["expectedHash"] = computed
			resp["docId"] = rec.DocID
			s.describe(ctx, rec, computed, resp)
			return s.status(ctx, rec, resp)
		}
	}

	// Unrecognised bytes, or no bytes at all. Fall back to the id.
	if id == "" {
		resp["status"] = "NOT_ISSUED"
		resp["message"] = "this file carries no registry marker and its hash is not on record, so it was never issued by any department"
		return nil
	}

	expectedRaw, known, err := s.reader.CurrentHashOf(ctx, id)
	if err != nil {
		return err
	}
	if !known {
		resp["status"] = "NOT_ISSUED"
		resp["message"] = fmt.Sprintf("no document with id %s has ever been issued", id)
		return nil
	}

	expected := "0x" + hex.EncodeToString(expectedRaw[:])
	resp["expectedHash"] = expected

	rec, err := s.reader.Verify(ctx, expectedRaw)
	if err != nil {
		return err
	}
	s.describe(ctx, rec, expected, resp)

	// Scanning an outdated QR resolves to whatever replaced it; say so rather
	// than silently answering about a different document than the one asked for.
	if rec.DocID != "" && rec.DocID != id {
		resp["resolvedDocId"] = rec.DocID
	}

	if computed != "" {
		// The id is on file but these bytes are not the bytes we anchored. That is
		// tampering, and both hashes are in the response to prove it.
		resp["status"] = "TAMPERED"
		resp["message"] = fmt.Sprintf("document %s exists, but this file does not match the hash on record — it has been modified since it was issued", id)
		return nil
	}
	return s.status(ctx, rec, resp)
}

// describe copies the record's metadata onto the response.
func (s *Server) describe(ctx context.Context, rec chain.Record, hash string, resp map[string]any) {
	resp["docType"] = chain.DocTypeName(rec.DocType)
	resp["issuer"] = rec.Issuer
	resp["timestamp"] = rec.Timestamp
	resp["prevHash"] = rec.PrevHash

	// The citizen's name lives off chain; it is a convenience, not proof.
	if doc, ok, _ := s.store.ByHash(hash); ok {
		resp["citizen"] = doc.Citizen
		resp["originalFilename"] = doc.Filename
	}
}

// status turns the on-chain status into a verdict, and for a superseded document
// points at the version that replaced it.
func (s *Server) status(ctx context.Context, rec chain.Record, resp map[string]any) error {
	switch rec.Status {
	case chain.StatusValid:
		resp["status"] = "VALID"
		resp["message"] = "authentic and current"

	case chain.StatusSuperseded:
		resp["status"] = "SUPERSEDED"
		resp["message"] = "authentic, but a newer version of this document has been issued"

		// Follow the id index to whatever is current now, so a verifier can be
		// sent straight to the live version instead of a dead end.
		curRaw, known, err := s.reader.CurrentHashOf(ctx, rec.DocID)
		if err != nil {
			return err
		}
		if known {
			cur, err := s.reader.Verify(ctx, curRaw)
			if err != nil {
				return err
			}
			resp["supersededBy"] = map[string]any{
				"docId": cur.DocID,
				"hash":  "0x" + hex.EncodeToString(curRaw[:]),
			}
		}

	case chain.StatusRevoked:
		resp["status"] = "REVOKED"
		resp["message"] = "authentic, but revoked by the issuing department"

	default:
		resp["status"] = "UNKNOWN"
		resp["message"] = "the registry returned a status this build does not recognise"
	}
	return nil
}

func (s *Server) revoke(w http.ResponseWriter, r *http.Request) {
	audit := operationFromContext(r.Context())
	var body struct {
		DocHash string `json:"docHash"`
		Dept    string `json:"dept"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "expected a JSON body with docHash and dept")
		return
	}
	audit.DocHash = body.DocHash

	dept, _, ok := requestDepartment(w, r)
	if !ok {
		return
	}
	if body.Dept != "" && body.Dept != dept.Slug {
		writeErr(w, http.StatusForbidden, "cannot revoke for another department")
		return
	}
	hash, err := parseHash(body.DocHash)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	record, err := s.reader.Verify(r.Context(), hash)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	if !record.Found {
		writeErr(w, http.StatusNotFound, "no such credential")
		return
	}
	audit.DocID = record.DocID
	if document, found, _ := s.store.ByHash(body.DocHash); found {
		audit.Citizen = document.Citizen
	}
	if record.DocType != dept.DocType {
		writeErr(w, http.StatusForbidden, "credential is outside the authenticated department")
		return
	}

	txHash, err := s.chain.Revoke(r.Context(), dept, hash, dept.DocType)
	if err != nil {
		audit.Outcome = "FAILURE"
		writeErr(w, http.StatusForbidden, err.Error())
		return
	}
	audit.Transaction = txHash
	audit.Result = "REVOKED"
	writeJSON(w, http.StatusOK, map[string]any{"docHash": body.DocHash, "txHash": txHash, "status": "REVOKED"})
}

// supersede replaces a document with a corrected version: multipart upload of the
// new PDF plus old_hash, dept, doc_id and citizen. The old record stays on chain
// as SUPERSEDED — corrections add history rather than erasing it.
func (s *Server) supersede(w http.ResponseWriter, r *http.Request) {
	audit := operationFromContext(r.Context())
	pdf, filename, err := readUpload(w, r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	dept, _, ok := requestDepartment(w, r)
	if !ok {
		return
	}
	if supplied := r.FormValue("dept"); supplied != "" && supplied != dept.Slug {
		writeErr(w, http.StatusForbidden, "cannot supersede for another department")
		return
	}

	oldHash, err := parseHash(r.FormValue("old_hash"))
	audit.ReferenceHash = r.FormValue("old_hash")
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	oldRecord, err := s.reader.Verify(r.Context(), oldHash)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	if !oldRecord.Found {
		writeErr(w, http.StatusNotFound, "no such credential")
		return
	}
	if oldRecord.DocType != dept.DocType {
		writeErr(w, http.StatusForbidden, "credential is outside the authenticated department")
		return
	}
	oldDocument, oldDocumentFound, _ := s.store.ByHash(r.FormValue("old_hash"))

	docID := strings.TrimSpace(r.FormValue("doc_id"))
	citizen := strings.TrimSpace(r.FormValue("citizen"))
	var citizenAccountID *string
	if oldDocumentFound && oldDocument.CitizenAccountID != nil {
		citizen = oldDocument.Citizen
		citizenAccountID = oldDocument.CitizenAccountID
	} else {
		accountID := strings.TrimSpace(r.FormValue("citizen_account_id"))
		account, accountErr := s.store.CitizenAccountByID(accountID)
		if accountErr != nil || !account.Active {
			writeErr(w, http.StatusBadRequest, "a valid active citizen_account_id is required for an unlinked credential")
			return
		}
		citizen, citizenAccountID = account.DisplayName, &account.ID
	}
	audit.DocID, audit.Citizen = docID, citizen
	if docID == "" || citizen == "" {
		writeErr(w, http.StatusBadRequest, "doc_id and citizen are required")
		return
	}

	stamped, marked := pdfdoc.StampWithQR(pdf, docID, s.verificationURL(docID))
	sum := sha256.Sum256(stamped)
	newHash := "0x" + hex.EncodeToString(sum[:])
	audit.DocHash = newHash

	txHash, err := s.chain.Supersede(r.Context(), dept, oldHash, sum, docID, dept.DocType)
	if err != nil {
		audit.Outcome = "FAILURE"
		writeErr(w, http.StatusForbidden, err.Error())
		return
	}
	audit.Transaction = txHash

	doc := store.Document{
		DocHash: newHash, DocID: docID, DocType: chain.DocTypeName(dept.DocType),
		Citizen: citizen, Issuer: dept.Address, Filename: filename, TxHash: txHash,
		CitizenAccountID: citizenAccountID,
	}
	if err := s.store.Save(doc, stamped); err != nil {
		audit.Outcome = "PARTIAL_FAILURE"
		writeErr(w, http.StatusInternalServerError, fmt.Sprintf("anchored on chain (%s) but the off-chain save failed: %v", txHash, err))
		return
	}
	audit.Result = "VALID"

	writeJSON(w, http.StatusCreated, map[string]any{
		"docHash":     newHash,
		"oldHash":     r.FormValue("old_hash"),
		"txHash":      txHash,
		"docId":       docID,
		"citizen":     doc.Citizen,
		"downloadUrl": downloadURL(newHash),
		"stamped":     marked,
	})
}

// credentials lists a citizen's documents, each with its live on-chain status.
func (s *Server) credentials(w http.ResponseWriter, r *http.Request) {
	citizen := r.PathValue("citizen")
	_, docTypeName, ok := requestDepartment(w, r)
	if !ok {
		return
	}

	docs, err := s.store.ByCitizenAndDocType(citizen, docTypeName)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	type entry struct {
		store.Document
		Status string `json:"status"`
	}
	out := []entry{}
	for _, d := range docs {
		e := entry{Document: d, Status: "UNKNOWN"}
		if hash, err := parseHash(d.DocHash); err == nil {
			if rec, err := s.chain.Verify(r.Context(), hash); err == nil && rec.Found {
				e.Status = statusName(rec.Status)
			}
		}
		out = append(out, e)
	}

	writeJSON(w, http.StatusOK, map[string]any{"citizen": citizen, "documents": out})
}

func (s *Server) citizenCredentials(w http.ResponseWriter, r *http.Request) {
	account, ok := s.authenticatedCitizen(w, r)
	if !ok {
		return
	}
	docs, err := s.store.ByCitizenAccountID(account.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not load credentials")
		return
	}
	type entry struct {
		store.Document
		Status string `json:"status"`
	}
	out := []entry{}
	for _, doc := range docs {
		item := entry{Document: doc, Status: "UNKNOWN"}
		if hash, err := parseHash(doc.DocHash); err == nil {
			if record, err := s.reader.Verify(r.Context(), hash); err == nil && record.Found {
				item.Status = statusName(record.Status)
			}
		}
		out = append(out, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{"citizen": account.DisplayName, "documents": out})
}

func (s *Server) citizenDownload(w http.ResponseWriter, r *http.Request) {
	account, ok := s.authenticatedCitizen(w, r)
	if !ok {
		return
	}
	hash := r.PathValue("hash")
	document, found, err := s.store.ByHash(hash)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not load document")
		return
	}
	if !found {
		writeErr(w, http.StatusNotFound, "no such document")
		return
	}
	if document.CitizenAccountID == nil || *document.CitizenAccountID != account.ID {
		writeErr(w, http.StatusForbidden, "document does not belong to this citizen account")
		return
	}
	pdf, found, err := s.store.PDF(hash)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not load document")
		return
	}
	if !found {
		writeErr(w, http.StatusNotFound, "no such document")
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, document.Filename))
	w.Write(pdf)
}

func (s *Server) authenticatedCitizen(w http.ResponseWriter, r *http.Request) (store.CitizenAccount, bool) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeErr(w, http.StatusInternalServerError, "authenticated user missing from context")
		return store.CitizenAccount{}, false
	}
	if _, err := s.store.UserProfileByID(user.ID); err == nil {
		writeErr(w, http.StatusConflict, "identity is linked to both government and citizen accounts")
		return store.CitizenAccount{}, false
	} else if !errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusInternalServerError, "could not load application account")
		return store.CitizenAccount{}, false
	}
	account, err := s.store.CitizenAccountBySupabaseUserID(user.ID)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusForbidden, "no citizen account is linked to this identity")
		return store.CitizenAccount{}, false
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not load citizen account")
		return store.CitizenAccount{}, false
	}
	if !account.Active {
		writeErr(w, http.StatusForbidden, "citizen account is inactive")
		return store.CitizenAccount{}, false
	}
	return account, true
}

func (s *Server) download(w http.ResponseWriter, r *http.Request) {
	_, docTypeName, authorized := requestDepartment(w, r)
	if !authorized {
		return
	}
	hash := r.PathValue("hash")
	document, ok, err := s.store.ByHash(hash)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		writeErr(w, http.StatusNotFound, "no such document")
		return
	}
	if document.DocType != docTypeName {
		writeErr(w, http.StatusForbidden, "document is outside the authenticated department")
		return
	}
	pdf, ok, err := s.store.PDF(hash)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		writeErr(w, http.StatusNotFound, "no such document")
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Write(pdf)
}

// --- helpers ----------------------------------------------------------------

// downloadURL is where the stamped PDF can be fetched. Relative, so it works
// whatever host the frontend is served from.
func downloadURL(docHash string) string {
	return "/documents/" + docHash + "/download"
}

func statusName(s uint8) string {
	switch s {
	case chain.StatusValid:
		return "VALID"
	case chain.StatusSuperseded:
		return "SUPERSEDED"
	case chain.StatusRevoked:
		return "REVOKED"
	}
	return "UNKNOWN"
}

func parseHash(s string) ([32]byte, error) {
	var h [32]byte
	b, err := hex.DecodeString(strings.TrimPrefix(s, "0x"))
	if err != nil || len(b) != 32 {
		return h, fmt.Errorf("%q is not a 32-byte hex hash", s)
	}
	copy(h[:], b)
	return h, nil
}

func readUpload(w http.ResponseWriter, r *http.Request) ([]byte, string, error) {
	// Allow bounded multipart framing in addition to the file itself, then enforce
	// the exact file limit below by reading one byte past it.
	r.Body = http.MaxBytesReader(w, r.Body, maxUpload+(1<<20))
	if err := r.ParseMultipartForm(maxUpload); err != nil {
		return nil, "", fmt.Errorf("expected a multipart form upload: %w", err)
	}
	f, header, err := r.FormFile("file")
	if err != nil {
		return nil, "", fmt.Errorf("missing 'file' field: %w", err)
	}
	defer f.Close()

	pdf, err := io.ReadAll(io.LimitReader(f, maxUpload+1))
	if err != nil {
		return nil, "", err
	}
	if len(pdf) == 0 {
		return nil, "", fmt.Errorf("uploaded file is empty")
	}
	if len(pdf) > maxUpload {
		return nil, "", fmt.Errorf("uploaded file exceeds the 20 MiB limit")
	}
	return pdf, header.Filename, nil
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func requestDepartment(w http.ResponseWriter, r *http.Request) (chain.Department, string, bool) {
	principal, ok := rbac.PrincipalFromContext(r.Context())
	if !ok || principal.Profile.Department == nil {
		writeErr(w, http.StatusForbidden, "a department profile is required")
		return chain.Department{}, "", false
	}
	profileDepartment := principal.Profile.Department
	department, ok := chain.DepartmentBySlug(profileDepartment.ID)
	if !ok || department.DocType != profileDepartment.DocType {
		writeErr(w, http.StatusInternalServerError, "department configuration does not match the chain")
		return chain.Department{}, "", false
	}
	return department, chain.DocTypeName(department.DocType), true
}

func authorizeVerificationResponse(w http.ResponseWriter, r *http.Request, response map[string]any) bool {
	_, expectedDocType, ok := requestDepartment(w, r)
	if !ok {
		return false
	}
	actual, present := response["docType"].(string)
	if present && actual != "" && actual != expectedDocType {
		writeErr(w, http.StatusForbidden, "credential is outside the authenticated department")
		return false
	}
	return true
}

// cors keeps the frontend (a later step) able to call this from Vite's dev port.
func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
