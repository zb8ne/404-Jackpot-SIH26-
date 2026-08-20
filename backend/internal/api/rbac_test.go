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
	if err := s.UpsertCitizenAccount(store.CitizenAccount{ID: "citizen-1", DisplayName: "Citizen", Email: "citizen@example.test", Active: true}); err != nil {
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
		"dept": department, "doc_type": docType, "doc_id": "DOC-1", "citizen": "Citizen", "citizen_account_id": "citizen-1",
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
		{http.MethodGet, "/account"},
		{http.MethodGet, "/citizen/credentials"},
		{http.MethodGet, "/citizen/documents/hash/download"},
		{http.MethodGet, "/citizen/verification-requests"},
		{http.MethodPost, "/citizen/verification-requests/request-id/decision"},
		{http.MethodGet, "/citizens"},
		{http.MethodPost, "/issue"},
		{http.MethodPost, "/verify"},
		{http.MethodGet, "/verify/DOC-1"},
		{http.MethodPost, "/revoke"},
		{http.MethodPost, "/supersede"},
		{http.MethodGet, "/credentials/Citizen"},
		{http.MethodGet, "/department/credentials"},
		{http.MethodGet, "/documents/hash/download"},
		{http.MethodGet, "/citizen-accounts"},
		{http.MethodPost, "/verification-requests"},
		{http.MethodGet, "/verification-requests"},
		{http.MethodGet, "/verification-requests/request-id"},
		{http.MethodPost, "/verification-requests/request-id/complete"},
		{http.MethodGet, "/development/notifications/request-id"},
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
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, authorizedRequest(http.MethodGet, "/department/credentials", nil, ""))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"docId":"DOC-1"`) {
		t.Fatalf("department credentials status = %d, body = %s", response.Code, response.Body.String())
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
		"doc_id": "BC-NEW", "citizen": "Citizen", "citizen_account_id": "citizen-1",
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

func TestGovernmentProfileCannotUseCitizenOwnedRoutes(t *testing.T) {
	handler := rbacHandler(t, "ADMIN", "birth")
	for _, path := range []string{"/citizen/credentials", "/citizen/verification-requests"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, authorizedRequest(http.MethodGet, path, nil, ""))
		if response.Code != http.StatusForbidden {
			t.Fatalf("GET %s status=%d, want 403", path, response.Code)
		}
	}
}

func TestAccountResolvesGovernmentProfile(t *testing.T) {
	handler := rbacHandler(t, "ADMIN", "birth")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authorizedRequest(http.MethodGet, "/account", nil, ""))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"accountType":"GOVERNMENT"`) ||
		!strings.Contains(response.Body.String(), `"email":"profile@example.gov"`) ||
		strings.Contains(response.Body.String(), "jwt@example.gov") {
		t.Fatalf("/account did not use backend government profile: %s", response.Body.String())
	}
}

func TestAccountResolvesCitizenProfile(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	supabaseID := "citizen-user"
	if err := s.UpsertCitizenAccount(store.CitizenAccount{
		ID: "citizen-1", SupabaseUserID: &supabaseID, DisplayName: "Citizen Name",
		Email: "profile@example.test", Active: true,
	}); err != nil {
		t.Fatal(err)
	}
	registry := &apiRegistry{records: map[[32]byte]chain.Record{}, current: map[string][32]byte{}}
	handler := newHandler(registry, s, apiVerifier{user: &auth.User{ID: supabaseID, Email: "jwt@example.test"}}, "0xcontract")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authorizedRequest(http.MethodGet, "/account", nil, ""))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"accountType":"CITIZEN"`) ||
		!strings.Contains(response.Body.String(), `"displayName":"Citizen Name"`) ||
		strings.Contains(response.Body.String(), "jwt@example.test") {
		t.Fatalf("/account did not use backend citizen profile: %s", response.Body.String())
	}
}

func TestAccountRejectsMissingAmbiguousAndInactiveProfiles(t *testing.T) {
	for _, test := range []struct {
		name       string
		government *store.UserProfile
		citizen    *store.CitizenAccount
		wantStatus int
	}{
		{name: "missing", wantStatus: http.StatusForbidden},
		{
			name:       "ambiguous",
			government: &store.UserProfile{SupabaseUserID: "user-1", Email: "admin@example.gov", DisplayName: "Admin", Role: "ADMIN", Active: true},
			citizen:    &store.CitizenAccount{ID: "citizen-1", DisplayName: "Citizen", Email: "citizen@example.test", Active: true},
			wantStatus: http.StatusConflict,
		},
		{
			name:       "inactive citizen",
			citizen:    &store.CitizenAccount{ID: "citizen-1", DisplayName: "Citizen", Email: "citizen@example.test", Active: false},
			wantStatus: http.StatusForbidden,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			s, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			if test.government != nil {
				if err := s.UpsertUserProfileAudited(*test.government, "birth", store.AuditActor{ID: "test", Role: "SYSTEM"}); err != nil {
					t.Fatal(err)
				}
			}
			if test.citizen != nil {
				supabaseID := "user-1"
				test.citizen.SupabaseUserID = &supabaseID
				if err := s.UpsertCitizenAccount(*test.citizen); err != nil {
					t.Fatal(err)
				}
			}
			registry := &apiRegistry{records: map[[32]byte]chain.Record{}, current: map[string][32]byte{}}
			handler := newHandler(registry, s, apiVerifier{user: &auth.User{ID: "user-1"}}, "0xcontract")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, authorizedRequest(http.MethodGet, "/account", nil, ""))
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", response.Code, test.wantStatus, response.Body.String())
			}
		})
	}
}

func TestCitizenCanOnlyReadAndDownloadOwnCredentials(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	supabaseID := "citizen-user"
	for _, account := range []store.CitizenAccount{
		{ID: "citizen-1", SupabaseUserID: &supabaseID, DisplayName: "Citizen One", Email: "one@example.test", Active: true},
		{ID: "citizen-2", DisplayName: "Citizen Two", Email: "two@example.test", Active: true},
	} {
		if err := s.UpsertCitizenAccount(account); err != nil {
			t.Fatal(err)
		}
	}
	one, two := "citizen-1", "citizen-2"
	oneHash := "0x" + strings.Repeat("0", 63) + "1"
	twoHash := "0x" + strings.Repeat("0", 63) + "2"
	if err := s.Save(store.Document{DocHash: oneHash, DocID: "OWN-1", DocType: "birth_certificate", Citizen: "Citizen One", Filename: "own.pdf", CitizenAccountID: &one}, []byte("own pdf")); err != nil {
		t.Fatal(err)
	}
	if err := s.Save(store.Document{DocHash: twoHash, DocID: "OTHER-1", DocType: "birth_certificate", Citizen: "Citizen Two", Filename: "other.pdf", CitizenAccountID: &two}, []byte("other pdf")); err != nil {
		t.Fatal(err)
	}
	registry := &apiRegistry{records: map[[32]byte]chain.Record{}, current: map[string][32]byte{}}
	var anchored [32]byte
	anchored[31] = 1
	registry.records[anchored] = chain.Record{Found: true, DocID: "OWN-1", Status: chain.StatusValid}
	handler := newHandler(registry, s, apiVerifier{user: &auth.User{ID: supabaseID}}, "0xcontract")
	governmentResponse := httptest.NewRecorder()
	handler.ServeHTTP(governmentResponse, authorizedRequest(http.MethodGet, "/department/credentials", nil, ""))
	if governmentResponse.Code != http.StatusForbidden {
		t.Fatalf("citizen government-route status=%d, want 403", governmentResponse.Code)
	}

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authorizedRequest(http.MethodGet, "/citizen/credentials", nil, ""))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "OWN-1") || strings.Contains(response.Body.String(), "OTHER-1") {
		t.Fatalf("credential list was not ownership scoped: status=%d body=%s", response.Code, response.Body.String())
	}

	response = httptest.NewRecorder()
	handler.ServeHTTP(response, authorizedRequest(http.MethodGet, "/citizen/documents/"+oneHash+"/download", nil, ""))
	if response.Code != http.StatusOK || response.Body.String() != "own pdf" {
		t.Fatalf("own download status=%d body=%q", response.Code, response.Body.String())
	}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, authorizedRequest(http.MethodGet, "/citizen/documents/"+twoHash+"/download", nil, ""))
	if response.Code != http.StatusForbidden {
		t.Fatalf("other citizen download status=%d, want 403", response.Code)
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
