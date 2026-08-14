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

type Server struct {
	chain *chain.Client
	store *store.Store
}

func New(c *chain.Client, s *store.Store) http.Handler {
	srv := &Server{chain: c, store: s}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", srv.health)
	mux.HandleFunc("GET /departments", srv.departments)
	mux.HandleFunc("GET /citizens", srv.citizens)
	mux.HandleFunc("POST /issue", srv.issue)
	mux.HandleFunc("POST /verify", srv.verify)
	mux.HandleFunc("GET /verify/{docId}", srv.verifyByID)
	mux.HandleFunc("POST /revoke", srv.revoke)
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

// verify identifies an uploaded PDF by its embedded marker, then compares the
// hash the registry expects for that id against the hash of the bytes we were
// handed. Identifying by id first is what separates a document that was altered
// from one that was never issued.
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

	id, ok := docid.Extract(pdf)
	if !ok {
		resp["status"] = "NOT_ISSUED"
		resp["message"] = "this file carries no registry marker, so it was never issued by any department"
		writeJSON(w, http.StatusOK, resp)
		return
	}
	resp["docId"] = id

	if err := s.resolve(r.Context(), id, computed, resp); err != nil {
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

// resolve fills in the verdict for a docId. When computed is empty the caller had
// no file, so the hash comparison is skipped.
func (s *Server) resolve(ctx context.Context, id, computed string, resp map[string]any) error {
	expectedRaw, known, err := s.chain.CurrentHashOf(ctx, id)
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

	rec, err := s.chain.Verify(ctx, expectedRaw)
	if err != nil {
		return err
	}

	resp["docType"] = chain.DocTypeName(rec.DocType)
	resp["issuer"] = rec.Issuer
	resp["timestamp"] = rec.Timestamp
	resp["prevHash"] = rec.PrevHash

	// The citizen's name lives off chain; it is a convenience, not proof.
	if doc, ok, _ := s.store.ByHash(expected); ok {
		resp["citizen"] = doc.Citizen
		resp["originalFilename"] = doc.Filename
	}

	if computed != "" && !strings.EqualFold(computed, expected) {
		// The id is genuine and on file, but these bytes are not the bytes we
		// anchored. That is tampering, and we can prove it by showing both hashes.
		resp["status"] = "TAMPERED"
		resp["message"] = fmt.Sprintf("document %s exists, but this file does not match the hash on record — it has been modified since it was issued", id)
		return nil
	}

	switch rec.Status {
	case chain.StatusValid:
		resp["status"] = "VALID"
		resp["message"] = "authentic and current"
	case chain.StatusSuperseded:
		resp["status"] = "SUPERSEDED"
		resp["message"] = "authentic, but a newer version of this document has been issued"
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
