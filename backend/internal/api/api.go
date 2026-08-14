// Package api exposes the REST surface: issue a document, verify an uploaded
// PDF against the chain, and list what a citizen holds.
package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"credreg/backend/internal/chain"
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

// verify hashes an uploaded PDF and asks the chain what it knows about it.
func (s *Server) verify(w http.ResponseWriter, r *http.Request) {
	pdf, filename, err := readUpload(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	sum := sha256.Sum256(pdf)
	docHash := "0x" + hex.EncodeToString(sum[:])

	rec, err := s.chain.Verify(r.Context(), sum)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}

	resp := map[string]any{
		"docHash":  docHash,
		"filename": filename,
	}

	if !rec.Found {
		// The chain has never seen this hash. Either the file was altered after
		// issue, or it was never issued at all — we cannot tell the two apart
		// from the bytes alone, so say so.
		resp["status"] = "TAMPERED_OR_NOT_FOUND"
		resp["message"] = "no on-chain record for this file's hash — it was either never issued or has been modified since"
		writeJSON(w, http.StatusOK, resp)
		return
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
	}

	resp["docId"] = rec.DocID
	resp["docType"] = chain.DocTypeName(rec.DocType)
	resp["issuer"] = rec.Issuer
	resp["issuedAt"] = rec.Timestamp
	resp["prevHash"] = rec.PrevHash

	// The filename and citizen are off-chain conveniences, not proof.
	if doc, ok, _ := s.store.ByHash(docHash); ok {
		resp["citizen"] = doc.Citizen
		resp["originalFilename"] = doc.Filename
	}

	writeJSON(w, http.StatusOK, resp)
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
