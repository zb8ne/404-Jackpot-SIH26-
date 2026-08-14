// Package api exposes the REST surface: issue a document, verify an uploaded
// PDF against the chain, and list what a citizen holds.
package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"credreg/backend/internal/chain"
	"credreg/backend/internal/docid"
	"credreg/backend/internal/store"
)

const maxUpload = 20 << 20 // 20 MB, plenty for a demo certificate

// reader is the read side of the registry. Splitting it out keeps the verify
// state machine testable without an Anvil node behind it.
type reader interface {
	Verify(ctx context.Context, docHash [32]byte) (chain.Record, error)
	CurrentHashOf(ctx context.Context, docID string) ([32]byte, bool, error)
}

type Server struct {
	chain  *chain.Client
	reader reader
	store  *store.Store
}

func New(c *chain.Client, s *store.Store) http.Handler {
	srv := &Server{chain: c, reader: c, store: s}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", srv.health)
	mux.HandleFunc("GET /departments", srv.departments)
	mux.HandleFunc("GET /citizens", srv.citizens)
	mux.HandleFunc("POST /issue", srv.issue)
	mux.HandleFunc("POST /verify", srv.verify)
	mux.HandleFunc("GET /verify/{docId}", srv.verifyByID)
	mux.HandleFunc("POST /revoke", srv.revoke)
	mux.HandleFunc("POST /supersede", srv.supersede)
	mux.HandleFunc("GET /credentials/{citizen}", srv.credentials)
	mux.HandleFunc("GET /documents/{hash}/download", srv.download)

	return cors(mux)
}

// --- handlers ---------------------------------------------------------------

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":       true,
		"contract": s.chain.Address.Hex(),
	})
}

func (s *Server) departments(w http.ResponseWriter, r *http.Request) {
	type dept struct {
		chain.Department
		DocTypeName string `json:"docTypeName"`
	}
	out := []dept{}
	for _, d := range chain.Departments() {
		out = append(out, dept{Department: d, DocTypeName: chain.DocTypeName(d.DocType)})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) citizens(w http.ResponseWriter, r *http.Request) {
	names, err := s.store.Citizens()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, names)
}

// issue takes a multipart upload: file, dept, doc_id, doc_type, citizen.
// The dept field is the hardcoded department picker — there is no auth.
func (s *Server) issue(w http.ResponseWriter, r *http.Request) {
	pdf, filename, err := readUpload(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	deptSlug := r.FormValue("dept")
	dept, ok := chain.DepartmentBySlug(deptSlug)
	if !ok {
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("unknown department %q", deptSlug))
		return
	}

	docTypeName := r.FormValue("doc_type")
	docType, ok := chain.DocTypeByName(docTypeName)
	if !ok {
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("unknown doc_type %q", docTypeName))
		return
	}

	docID := strings.TrimSpace(r.FormValue("doc_id"))
	citizen := strings.TrimSpace(r.FormValue("citizen"))
	if docID == "" || citizen == "" {
		writeErr(w, http.StatusBadRequest, "doc_id and citizen are required")
		return
	}

	// Hash of the raw PDF bytes. Re-saving the file changes this — accepted
	// for the demo, it is what makes tampering detectable at all.
	sum := sha256.Sum256(pdf)
	docHash := "0x" + hex.EncodeToString(sum[:])

	// The contract is the authority on whether this department may issue this
	// document type. If the roles disagree, this call reverts and we surface it.
	txHash, err := s.chain.Issue(r.Context(), dept, sum, docID, docType)
	if err != nil {
		writeErr(w, http.StatusForbidden, err.Error())
		return
	}

	doc := store.Document{
		DocHash: docHash, DocID: docID, DocType: docTypeName, Citizen: citizen,
		Issuer: dept.Address, Filename: filename, TxHash: txHash,
	}
	if err := s.store.Save(doc, pdf); err != nil {
		writeErr(w, http.StatusInternalServerError, fmt.Sprintf("anchored on chain (%s) but the off-chain save failed: %v", txHash, err))
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"docHash": docHash,
		"txHash":  txHash,
		"issuer":  dept.Name,
		"docType": docTypeName,
		"docId":   docID,
		"citizen": citizen,
	})
}

// zeroHash is what an unknown docId resolves to, and what we report as the
// expected hash when there is nothing to expect.
const zeroHash = "0x" + "0000000000000000000000000000000000000000000000000000000000000000"

// verify hashes an uploaded PDF and reads the docId marker embedded in it, then
// hands both to resolve. Having the id as well as the hash is what separates a
// document that was altered from one that was never issued at all.
func (s *Server) verify(w http.ResponseWriter, r *http.Request) {
	pdf, filename, err := readUpload(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	sum := sha256.Sum256(pdf)
	computed := "0x" + hex.EncodeToString(sum[:])

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
	}

	if err := s.resolve(r.Context(), resp["docId"].(string), computed, resp); err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// verifyByID answers the same question from an id alone — what a QR scan hits
// when the verifier is holding a phone rather than the file. Without bytes to
// hash there is no computedHash, so the verdict can only be what the registry
// currently says about the document.
func (s *Server) verifyByID(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("docId")

	resp := map[string]any{
		"docId":        id,
		"expectedHash": zeroHash,
	}
	if err := s.resolve(r.Context(), id, "", resp); err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
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
	var body struct {
		DocHash string `json:"docHash"`
		Dept    string `json:"dept"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "expected a JSON body with docHash and dept")
		return
	}

	dept, ok := chain.DepartmentBySlug(body.Dept)
	if !ok {
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("unknown department %q", body.Dept))
		return
	}
	hash, err := parseHash(body.DocHash)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	txHash, err := s.chain.Revoke(r.Context(), dept, hash, dept.DocType)
	if err != nil {
		writeErr(w, http.StatusForbidden, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"docHash": body.DocHash, "txHash": txHash, "status": "REVOKED"})
}

// supersede replaces a document with a corrected version: multipart upload of the
// new PDF plus old_hash, dept, doc_id and citizen. The old record stays on chain
// as SUPERSEDED — corrections add history rather than erasing it.
func (s *Server) supersede(w http.ResponseWriter, r *http.Request) {
	pdf, filename, err := readUpload(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	dept, ok := chain.DepartmentBySlug(r.FormValue("dept"))
	if !ok {
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("unknown department %q", r.FormValue("dept")))
		return
	}

	oldHash, err := parseHash(r.FormValue("old_hash"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	docID := strings.TrimSpace(r.FormValue("doc_id"))
	citizen := strings.TrimSpace(r.FormValue("citizen"))
	if docID == "" || citizen == "" {
		writeErr(w, http.StatusBadRequest, "doc_id and citizen are required")
		return
	}

	sum := sha256.Sum256(pdf)
	newHash := "0x" + hex.EncodeToString(sum[:])

	txHash, err := s.chain.Supersede(r.Context(), dept, oldHash, sum, docID, dept.DocType)
	if err != nil {
		writeErr(w, http.StatusForbidden, err.Error())
		return
	}

	doc := store.Document{
		DocHash: newHash, DocID: docID, DocType: chain.DocTypeName(dept.DocType),
		Citizen: citizen, Issuer: dept.Address, Filename: filename, TxHash: txHash,
	}
	if err := s.store.Save(doc, pdf); err != nil {
		writeErr(w, http.StatusInternalServerError, fmt.Sprintf("anchored on chain (%s) but the off-chain save failed: %v", txHash, err))
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"docHash": newHash,
		"oldHash": r.FormValue("old_hash"),
		"txHash":  txHash,
		"docId":   docID,
		"citizen": citizen,
	})
}

// credentials lists a citizen's documents, each with its live on-chain status.
func (s *Server) credentials(w http.ResponseWriter, r *http.Request) {
	citizen := r.PathValue("citizen")

	docs, err := s.store.ByCitizen(citizen)
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

func (s *Server) download(w http.ResponseWriter, r *http.Request) {
	pdf, ok, err := s.store.PDF(r.PathValue("hash"))
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

func readUpload(r *http.Request) ([]byte, string, error) {
	if err := r.ParseMultipartForm(maxUpload); err != nil {
		return nil, "", fmt.Errorf("expected a multipart form upload: %w", err)
	}
	f, header, err := r.FormFile("file")
	if err != nil {
		return nil, "", fmt.Errorf("missing 'file' field: %w", err)
	}
	defer f.Close()

	pdf, err := io.ReadAll(io.LimitReader(f, maxUpload))
	if err != nil {
		return nil, "", err
	}
	if len(pdf) == 0 {
		return nil, "", fmt.Errorf("uploaded file is empty")
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

// cors keeps the frontend (a later step) able to call this from Vite's dev port.
func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
