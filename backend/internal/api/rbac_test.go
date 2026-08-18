package api

import (
	"bytes"
	"context"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"credreg/backend/internal/auth"
	"credreg/backend/internal/chain"
	"credreg/backend/internal/store"
)

type apiVerifier struct{ user *auth.User }

func (v apiVerifier) Verify(context.Context, string) (*auth.User, error) {
	if v.user == nil {
		return nil, errors.New("invalid token")
	}
	return v.user, nil
}

type apiRegistry struct {
	records      map[[32]byte]chain.Record
	current      map[string][32]byte
	issueErr     error
	revokeErr    error
	supersedeErr error
}

func (f *apiRegistry) Verify(_ context.Context, hash [32]byte) (chain.Record, error) {
	record, ok := f.records[hash]
	if !ok {
		return chain.Record{}, nil
	}
	record.Found = true
	return record, nil
}

func (f *apiRegistry) CurrentHashOf(_ context.Context, id string) ([32]byte, bool, error) {
	hash, ok := f.current[id]
	return hash, ok, nil
}

func (f *apiRegistry) Issue(_ context.Context, _ chain.Department, hash [32]byte, id string, docType uint8) (string, error) {
	if f.issueErr != nil {
		return "", f.issueErr
	}
	f.records[hash] = chain.Record{Found: true, DocID: id, DocType: docType, Status: chain.StatusValid}
	f.current[id] = hash
	return "0xtx", nil
}

func (f *apiRegistry) Supersede(_ context.Context, _ chain.Department, oldHash, newHash [32]byte, id string, docType uint8) (string, error) {
	if f.supersedeErr != nil {
		return "", f.supersedeErr
	}
	old := f.records[oldHash]
	old.Status = chain.StatusSuperseded
	f.records[oldHash] = old
	f.records[newHash] = chain.Record{Found: true, DocID: id, DocType: docType, Status: chain.StatusValid}
	return "0xtx", nil
}

func (f *apiRegistry) Revoke(_ context.Context, _ chain.Department, hash [32]byte, _ uint8) (string, error) {
	if f.revokeErr != nil {
		return "", f.revokeErr
	}
	record := f.records[hash]
	record.Status = chain.StatusRevoked
	f.records[hash] = record
	return "0xtx", nil
}

func rbacHandler(t *testing.T, role, department string) http.Handler {
	handler, _ := rbacFixture(t, role, department)
	return handler
}

func rbacFixture(t *testing.T, role, department string) (http.Handler, *apiRegistry) {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	profile := store.UserProfile{
		SupabaseUserID: "user-1", Email: "profile@example.gov",
		DisplayName: "Profile Name", Role: role, Active: true,
	}
	if err := s.UpsertUserProfileAudited(profile, department, store.AuditActor{ID: "test", Role: "SYSTEM"}); err != nil {
		t.Fatal(err)
	}
	registry := &apiRegistry{records: map[[32]byte]chain.Record{}, current: map[string][32]byte{}}
	return newHandler(registry, s, apiVerifier{user: &auth.User{ID: "user-1", Email: "jwt@example.gov"}}, "0xcontract"), registry
}

func authorizedRequest(method, target string, body *bytes.Buffer, contentType string) *http.Request {
	var req *http.Request
	if body == nil {
		req = httptest.NewRequest(method, target, nil)
	} else {
		req = httptest.NewRequest(method, target, body)
	}
	req.Header.Set("Authorization", "Bearer token")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return req
}

func issueBody(t *testing.T, department, docType string) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "document.pdf")
	if err != nil {
		t.Fatal(err)
	}
	part.Write([]byte("%PDF-1.4\nminimal"))
	for key, value := range map[string]string{
		"dept": department, "doc_type": docType, "doc_id": "DOC-1", "citizen": "Citizen",
	} {
		writer.WriteField(key, value)
	}
	writer.Close()
	return body, writer.FormDataContentType()
}

func TestProtectedRoutesRejectUnauthenticatedRequests(t *testing.T) {
	handler := rbacHandler(t, "ADMIN", "birth")
	for _, endpoint := range []struct{ method, path string }{
		{http.MethodGet, "/me"},
		{http.MethodGet, "/citizens"},
		{http.MethodPost, "/issue"},
		{http.MethodPost, "/verify"},
		{http.MethodGet, "/verify/DOC-1"},
		{http.MethodPost, "/revoke"},
		{http.MethodPost, "/supersede"},
		{http.MethodGet, "/credentials/Citizen"},
		{http.MethodGet, "/documents/hash/download"},
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(endpoint.method, endpoint.path, nil))
		if response.Code != http.StatusUnauthorized {
			t.Errorf("%s %s status = %d, want 401", endpoint.method, endpoint.path, response.Code)
		}
	}
}

func TestControllerCannotPerformCredentialOperations(t *testing.T) {
	handler := rbacHandler(t, "CONTROLLER", "")
	for _, endpoint := range []struct{ method, path string }{
		{http.MethodPost, "/issue"},
		{http.MethodPost, "/verify"},
		{http.MethodGet, "/verify/DOC-1"},
		{http.MethodPost, "/supersede"},
		{http.MethodPost, "/revoke"},
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, authorizedRequest(endpoint.method, endpoint.path, nil, ""))
		if response.Code != http.StatusForbidden {
			t.Errorf("%s %s status = %d, want 403", endpoint.method, endpoint.path, response.Code)
		}
	}
}

func TestOfficialCanIssueAndVerifyButCannotMutateLifecycle(t *testing.T) {
	handler := rbacHandler(t, "OFFICIAL", "birth")
	body, contentType := issueBody(t, "birth", "birth_certificate")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authorizedRequest(http.MethodPost, "/issue", body, contentType))
	if response.Code != http.StatusCreated {
		t.Fatalf("issue status = %d, body = %s", response.Code, response.Body.String())
	}

	verifyBody := &bytes.Buffer{}
	writer := multipart.NewWriter(verifyBody)
	part, _ := writer.CreateFormFile("file", "unknown.pdf")
	part.Write([]byte("unknown document"))
	writer.Close()
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, authorizedRequest(http.MethodPost, "/verify", verifyBody, writer.FormDataContentType()))
	if response.Code != http.StatusOK {
		t.Fatalf("verify status = %d, body = %s", response.Code, response.Body.String())
	}

	for _, path := range []string{"/supersede", "/revoke"} {
		response = httptest.NewRecorder()
		handler.ServeHTTP(response, authorizedRequest(http.MethodPost, path, nil, ""))
		if response.Code != http.StatusForbidden {
			t.Errorf("POST %s status = %d, want 403", path, response.Code)
		}
	}
}

func TestAdminCannotSelectAnotherDepartment(t *testing.T) {
	handler := rbacHandler(t, "ADMIN", "birth")
	body, contentType := issueBody(t, "transport", "driving_licence")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authorizedRequest(http.MethodPost, "/issue", body, contentType))
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body = %s", response.Code, response.Body.String())
	}
}

func TestDepartmentScopedVerificationRejectsAnotherDepartment(t *testing.T) {
	handler, registry := rbacFixture(t, "OFFICIAL", "birth")
	transportFile := []byte("transport credential\nCREDREG-DOCID:DL-1\n")
	hash, _ := hashOf(transportFile)
	registry.records[hash] = chain.Record{
		Found: true, DocID: "DL-1", DocType: chain.DocDrivingLicence, Status: chain.StatusValid,
	}
	registry.current["DL-1"] = hash

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "licence.pdf")
	part.Write(transportFile)
	writer.Close()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authorizedRequest(http.MethodPost, "/verify", body, writer.FormDataContentType()))
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body = %s", response.Code, response.Body.String())
	}
}

func TestAdminCanRevokeWithinOwnDepartment(t *testing.T) {
	handler, registry := rbacFixture(t, "ADMIN", "birth")
	var hash [32]byte
	hash[31] = 1
	registry.records[hash] = chain.Record{
		Found: true, DocID: "BC-1", DocType: chain.DocBirthCertificate, Status: chain.StatusValid,
	}
	body := bytes.NewBufferString(`{"docHash":"0x` + strings.Repeat("0", 62) + `01","dept":"birth"}`)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authorizedRequest(http.MethodPost, "/revoke", body, "application/json"))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if registry.records[hash].Status != chain.StatusRevoked {
		t.Fatal("registry record was not revoked")
	}
}

func TestAdminCanSupersedeWithinOwnDepartment(t *testing.T) {
	handler, registry := rbacFixture(t, "ADMIN", "birth")
	var oldHash [32]byte
	oldHash[31] = 2
	registry.records[oldHash] = chain.Record{
		Found: true, DocID: "BC-OLD", DocType: chain.DocBirthCertificate, Status: chain.StatusValid,
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "replacement.pdf")
	part.Write([]byte("%PDF-1.4\nreplacement"))
	for key, value := range map[string]string{
		"dept": "birth", "old_hash": "0x" + strings.Repeat("0", 62) + "02",
		"doc_id": "BC-NEW", "citizen": "Citizen",
	} {
		writer.WriteField(key, value)
	}
	writer.Close()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authorizedRequest(http.MethodPost, "/supersede", body, writer.FormDataContentType()))
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if registry.records[oldHash].Status != chain.StatusSuperseded {
		t.Fatal("old registry record was not superseded")
	}
}

func TestAdminCannotRevokeAnotherDepartmentCredential(t *testing.T) {
	handler, registry := rbacFixture(t, "ADMIN", "birth")
	var hash [32]byte
	hash[31] = 3
	registry.records[hash] = chain.Record{
		Found: true, DocID: "DL-OTHER", DocType: chain.DocDrivingLicence, Status: chain.StatusValid,
	}
	body := bytes.NewBufferString(`{"docHash":"0x` + strings.Repeat("0", 62) + `03","dept":"birth"}`)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authorizedRequest(http.MethodPost, "/revoke", body, "application/json"))
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body = %s", response.Code, response.Body.String())
	}
}

func TestMeUsesBackendOwnedProfile(t *testing.T) {
	handler := rbacHandler(t, "ADMIN", "birth")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authorizedRequest(http.MethodGet, "/me", nil, ""))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"email":"profile@example.gov"`) ||
		strings.Contains(response.Body.String(), "jwt@example.gov") {
		t.Fatalf("/me did not use backend profile: %s", response.Body.String())
	}
}

func TestHealthAndDepartmentsRemainPublic(t *testing.T) {
	handler := rbacHandler(t, "ADMIN", "birth")
	for _, path := range []string{"/health", "/departments"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusOK {
			t.Errorf("GET %s status = %d, want 200", path, response.Code)
		}
	}
}
