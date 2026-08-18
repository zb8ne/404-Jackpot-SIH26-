package api

import (
	"bytes"
	"encoding/json"
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

func auditFixture(t *testing.T, role, department string) (http.Handler, *store.Store, *apiRegistry) {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "audit.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	profile := store.UserProfile{
		SupabaseUserID: "audit-user", Email: "audit@example.gov", DisplayName: "Audit User",
		Role: role, Active: true,
	}
	if err := s.UpsertUserProfileAudited(profile, department, store.AuditActor{ID: "test", Role: "SYSTEM"}); err != nil {
		t.Fatal(err)
	}
	registry := &apiRegistry{records: map[[32]byte]chain.Record{}, current: map[string][32]byte{}}
	handler := newHandler(registry, s, apiVerifier{user: &auth.User{ID: "audit-user"}}, "0xcontract")
	return handler, s, registry
}

func TestSuccessfulIssueCreatesAuditEvent(t *testing.T) {
	handler, s, _ := auditFixture(t, "OFFICIAL", "birth")
	body, contentType := issueBody(t, "birth", "birth_certificate")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authorizedRequest(http.MethodPost, "/issue", body, contentType))
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	page, err := s.AuditEvents(store.AuditQuery{Limit: 10, Action: "ISSUE"})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Events) != 1 || page.Events[0].Action != "ISSUE" || page.Events[0].Outcome != "SUCCESS" {
		t.Fatalf("unexpected events: %#v", page.Events)
	}
	if page.Events[0].Credential.DocID == nil || *page.Events[0].Credential.DocID != "DOC-1" {
		t.Fatalf("document ID not audited: %#v", page.Events[0])
	}
}

func TestForbiddenOperationCreatesDeniedNotSuccess(t *testing.T) {
	handler, s, _ := auditFixture(t, "OFFICIAL", "birth")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authorizedRequest(http.MethodPost, "/revoke", bytes.NewBufferString(`{"docHash":"x"}`), "application/json"))
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d", response.Code)
	}
	page, _ := s.AuditEvents(store.AuditQuery{Limit: 10, Action: "REVOKE"})
	if len(page.Events) != 1 || page.Events[0].Outcome != "DENIED" {
		t.Fatalf("unexpected events: %#v", page.Events)
	}
}

func TestVerificationVerdictCreatesSuccessfulAuditEvent(t *testing.T) {
	handler, s, _ := auditFixture(t, "OFFICIAL", "birth")
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "unknown.pdf")
	_, _ = part.Write([]byte("unknown document"))
	_ = writer.Close()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authorizedRequest(http.MethodPost, "/verify", body, writer.FormDataContentType()))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	page, _ := s.AuditEvents(store.AuditQuery{Limit: 10, Action: "VERIFY_FILE"})
	if len(page.Events) != 1 || page.Events[0].Outcome != "SUCCESS" || page.Events[0].Result != "NOT_ISSUED" {
		t.Fatalf("negative verdict was not audited as success: %#v", page.Events)
	}
}

func TestSuccessfulRevokeCreatesAuditEvent(t *testing.T) {
	handler, s, registry := auditFixture(t, "ADMIN", "birth")
	var hash [32]byte
	hash[31] = 7
	registry.records[hash] = chain.Record{Found: true, DocID: "BC-7", DocType: chain.DocBirthCertificate, Status: chain.StatusValid}
	body := bytes.NewBufferString(`{"docHash":"0x` + strings.Repeat("0", 62) + `07","dept":"birth"}`)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authorizedRequest(http.MethodPost, "/revoke", body, "application/json"))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	page, _ := s.AuditEvents(store.AuditQuery{Limit: 10, Action: "REVOKE"})
	if len(page.Events) != 1 || page.Events[0].Outcome != "SUCCESS" || page.Events[0].Result != "REVOKED" || page.Events[0].Credential.TransactionHash == nil {
		t.Fatalf("unexpected revoke audit: %#v", page.Events)
	}
}

func TestSuccessfulSupersedeCreatesAuditEvent(t *testing.T) {
	handler, s, registry := auditFixture(t, "ADMIN", "birth")
	var oldHash [32]byte
	oldHash[31] = 8
	registry.records[oldHash] = chain.Record{Found: true, DocID: "BC-OLD", DocType: chain.DocBirthCertificate, Status: chain.StatusValid}
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "replacement.pdf")
	_, _ = part.Write([]byte("%PDF-1.4\nreplacement"))
	for key, value := range map[string]string{
		"dept": "birth", "old_hash": "0x" + strings.Repeat("0", 62) + "08",
		"doc_id": "BC-NEW", "citizen": "Citizen",
	} {
		_ = writer.WriteField(key, value)
	}
	_ = writer.Close()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authorizedRequest(http.MethodPost, "/supersede", body, writer.FormDataContentType()))
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	page, _ := s.AuditEvents(store.AuditQuery{Limit: 10, Action: "SUPERSEDE"})
	if len(page.Events) != 1 || page.Events[0].Outcome != "SUCCESS" || page.Events[0].Credential.ReferenceHash == nil || page.Events[0].Credential.TransactionHash == nil {
		t.Fatalf("unexpected supersede audit: %#v", page.Events)
	}
}

func TestBlockchainSuccessAndSQLiteSaveFailureIsPartialFailure(t *testing.T) {
	handler, s, _ := auditFixture(t, "OFFICIAL", "birth")
	for attempt := 0; attempt < 2; attempt++ {
		body, contentType := issueBody(t, "birth", "birth_certificate")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, authorizedRequest(http.MethodPost, "/issue", body, contentType))
		if attempt == 0 && response.Code != http.StatusCreated {
			t.Fatalf("first issue status = %d, body = %s", response.Code, response.Body.String())
		}
		if attempt == 1 && response.Code != http.StatusInternalServerError {
			t.Fatalf("second issue status = %d, body = %s", response.Code, response.Body.String())
		}
	}
	page, _ := s.AuditEvents(store.AuditQuery{Limit: 10, Action: "ISSUE", Outcome: "PARTIAL_FAILURE"})
	if len(page.Events) != 1 || page.Events[0].Credential.TransactionHash == nil {
		t.Fatalf("unexpected partial failure audit: %#v", page.Events)
	}
}

func TestBlockchainWriteFailureIsFailureNotDenied(t *testing.T) {
	handler, s, registry := auditFixture(t, "OFFICIAL", "birth")
	registry.issueErr = errors.New("rpc transaction submission failed")
	body, contentType := issueBody(t, "birth", "birth_certificate")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authorizedRequest(http.MethodPost, "/issue", body, contentType))
	if response.Code != http.StatusForbidden {
		t.Fatalf("existing HTTP contract changed: status = %d", response.Code)
	}
	page, _ := s.AuditEvents(store.AuditQuery{Limit: 10, Action: "ISSUE"})
	if len(page.Events) != 1 || page.Events[0].Outcome != "FAILURE" {
		t.Fatalf("blockchain failure misclassified: %#v", page.Events)
	}
}

func TestOversizedUploadIsExplicitlyRejected(t *testing.T) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "oversized.pdf")
	_, _ = part.Write(bytes.Repeat([]byte{'x'}, maxUpload+1))
	_ = writer.Close()
	request := httptest.NewRequest(http.MethodPost, "/verify", body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()
	_, _, err := readUpload(response, request)
	if err == nil || !strings.Contains(err.Error(), "exceeds the 20 MiB limit") {
		t.Fatalf("oversized upload error = %v", err)
	}
}

func TestAuditAPIControllerAndAdminScoping(t *testing.T) {
	controller, controllerStore, _ := auditFixture(t, "CONTROLLER", "")
	for _, department := range []string{"birth", "transport"} {
		_, err := controllerStore.AppendAuditEvent(store.AuditEvent{
			Actor:      store.AuditActor{ID: "actor", Role: "ADMIN"},
			Department: &store.AuditDepartment{ID: department, Name: department},
			Action:     "ISSUE", Outcome: "SUCCESS", HTTPStatus: 201,
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	response := httptest.NewRecorder()
	controller.ServeHTTP(response, authorizedRequest(http.MethodGet, "/audit-events?limit=10", nil, ""))
	if response.Code != http.StatusOK || strings.Count(response.Body.String(), `"action":"ISSUE"`) != 2 {
		t.Fatalf("controller response %d: %s", response.Code, response.Body.String())
	}

	admin, adminStore, _ := auditFixture(t, "ADMIN", "birth")
	for _, department := range []string{"birth", "transport"} {
		_, _ = adminStore.AppendAuditEvent(store.AuditEvent{
			Actor:      store.AuditActor{ID: "actor", Role: "ADMIN"},
			Department: &store.AuditDepartment{ID: department, Name: department},
			Action:     "ISSUE", Outcome: "SUCCESS", HTTPStatus: 201,
		})
	}
	response = httptest.NewRecorder()
	admin.ServeHTTP(response, authorizedRequest(http.MethodGet, "/audit-events?limit=10", nil, ""))
	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), `"id":"transport"`) {
		t.Fatalf("admin scope response %d: %s", response.Code, response.Body.String())
	}
	response = httptest.NewRecorder()
	admin.ServeHTTP(response, authorizedRequest(http.MethodGet, "/audit-events?department=transport", nil, ""))
	if response.Code != http.StatusForbidden {
		t.Fatalf("cross-department status = %d", response.Code)
	}
}

func TestMonitoringIsControllerOnly(t *testing.T) {
	controller, s, _ := auditFixture(t, "CONTROLLER", "")
	_, _ = s.AppendAuditEvent(store.AuditEvent{
		Actor:      store.AuditActor{ID: "official", Role: "OFFICIAL"},
		Department: &store.AuditDepartment{ID: "birth", Name: "Birth Registration Dept"},
		Action:     "VERIFY_FILE", Outcome: "SUCCESS", Result: "TAMPERED", HTTPStatus: 200,
	})
	response := httptest.NewRecorder()
	controller.ServeHTTP(response, authorizedRequest(http.MethodGet, "/monitoring/overview", nil, ""))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"auditIntegrity"`) {
		t.Fatalf("controller monitoring %d: %s", response.Code, response.Body.String())
	}
	var overview struct {
		Operations          map[string]operationCounts `json:"operations"`
		VerificationResults map[string]int             `json:"verificationResults"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &overview); err != nil {
		t.Fatal(err)
	}
	if overview.Operations["verify"].Success != 1 || overview.VerificationResults["TAMPERED"] != 1 {
		t.Fatalf("incorrect monitoring statistics: %#v", overview)
	}
	official, _, _ := auditFixture(t, "OFFICIAL", "birth")
	response = httptest.NewRecorder()
	official.ServeHTTP(response, authorizedRequest(http.MethodGet, "/monitoring/overview", nil, ""))
	if response.Code != http.StatusForbidden {
		t.Fatalf("official monitoring status = %d", response.Code)
	}
	response = httptest.NewRecorder()
	official.ServeHTTP(response, authorizedRequest(http.MethodGet, "/audit-events", nil, ""))
	if response.Code != http.StatusForbidden {
		t.Fatalf("official audit status = %d", response.Code)
	}
}
