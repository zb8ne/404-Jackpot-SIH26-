package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"credreg/backend/internal/auth"
	"credreg/backend/internal/chain"
	"credreg/backend/internal/store"
)

type consentFixture struct {
	handler       http.Handler
	store         *store.Store
	registry      *apiRegistry
	notifications *developmentNotifier
}

func newConsentFixture(t *testing.T, role, department string) consentFixture {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "consent.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	profile := store.UserProfile{SupabaseUserID: "user-1", Email: "official@example.gov", DisplayName: "Official", Role: role, Active: true}
	if err := s.UpsertUserProfileAudited(profile, department, store.AuditActor{ID: "test", Role: "SYSTEM"}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertCitizenAccount(store.CitizenAccount{ID: "citizen-1", DisplayName: "Citizen", Email: "citizen@example.test", Active: true}); err != nil {
		t.Fatal(err)
	}
	registry := &apiRegistry{records: map[[32]byte]chain.Record{}, current: map[string][32]byte{}}
	notifications := newDevelopmentNotifier()
	handler := newHandlerConfigured(registry, s, apiVerifier{user: &auth.User{ID: "user-1"}}, "0xcontract", "http://web.test", notifications)
	return consentFixture{handler, s, registry, notifications}
}

func (f consentFixture) linkCredential(t *testing.T, docType uint8, linked bool) [32]byte {
	t.Helper()
	hash := sha256.Sum256([]byte("credential"))
	f.registry.records[hash] = chain.Record{Found: true, DocID: "DOC-1", DocType: docType, Status: chain.StatusValid}
	f.registry.current["DOC-1"] = hash
	hashText := "0x" + hex.EncodeToString(hash[:])
	doc := store.Document{DocHash: hashText, DocID: "DOC-1", DocType: chain.DocTypeName(docType), Citizen: "Citizen", Issuer: "issuer", Filename: "doc.pdf", TxHash: "tx"}
	if linked {
		id := "citizen-1"
		doc.CitizenAccountID = &id
	}
	if err := f.store.Save(doc, []byte("credential")); err != nil {
		t.Fatal(err)
	}
	return hash
}

func createRequest(t *testing.T, f consentFixture) string {
	t.Helper()
	response := httptest.NewRecorder()
	body := bytes.NewBufferString(`{"documentId":"DOC-1","purpose":"employment check"}`)
	f.handler.ServeHTTP(response, authorizedRequest(http.MethodPost, "/verification-requests", body, "application/json"))
	if response.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", response.Code, response.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	return created.ID
}

func consentToken(t *testing.T, f consentFixture, id string) string {
	t.Helper()
	response := httptest.NewRecorder()
	f.handler.ServeHTTP(response, authorizedRequest(http.MethodGet, "/development/notifications/"+id, nil, ""))
	if response.Code != http.StatusOK {
		t.Fatalf("notification status=%d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		ConsentURL string `json:"consentUrl"`
	}
	json.Unmarshal(response.Body.Bytes(), &body)
	parsed, err := url.Parse(body.ConsentURL)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimPrefix(parsed.Path, "/consent/")
}

func TestConsentRequestApproveAndCompleteCurrentState(t *testing.T) {
	f := newConsentFixture(t, "OFFICIAL", "birth")
	hash := f.linkCredential(t, chain.DocBirthCertificate, true)
	id := createRequest(t, f)
	token := consentToken(t, f, id)
	req, err := f.store.VerificationRequestByID(id)
	if err != nil {
		t.Fatal(err)
	}
	if req.ApprovalTokenHash == nil || *req.ApprovalTokenHash == token {
		t.Fatal("raw token was persisted")
	}
	response := httptest.NewRecorder()
	f.handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/consent/"+token, nil))
	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), "citizen@example.test") {
		t.Fatalf("consent context=%d %s", response.Code, response.Body.String())
	}
	response = httptest.NewRecorder()
	f.handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/consent/"+token+"/approve", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("approve=%d %s", response.Code, response.Body.String())
	}
	record := f.registry.records[hash]
	record.Status = chain.StatusRevoked
	f.registry.records[hash] = record
	response = httptest.NewRecorder()
	f.handler.ServeHTTP(response, authorizedRequest(http.MethodPost, "/verification-requests/"+id+"/complete", nil, ""))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"status":"REVOKED"`) {
		t.Fatalf("complete=%d %s", response.Code, response.Body.String())
	}
	response = httptest.NewRecorder()
	f.handler.ServeHTTP(response, authorizedRequest(http.MethodPost, "/verification-requests/"+id+"/complete", nil, ""))
	if response.Code != http.StatusConflict {
		t.Fatalf("double complete=%d", response.Code)
	}
}

func TestConsentDenyAndConflictingDecision(t *testing.T) {
	f := newConsentFixture(t, "ADMIN", "birth")
	f.linkCredential(t, chain.DocBirthCertificate, true)
	id := createRequest(t, f)
	token := consentToken(t, f, id)
	response := httptest.NewRecorder()
	f.handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/consent/"+token+"/deny", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("deny=%d", response.Code)
	}
	response = httptest.NewRecorder()
	f.handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/consent/"+token+"/deny", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("idempotent deny=%d", response.Code)
	}
	response = httptest.NewRecorder()
	f.handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/consent/"+token+"/approve", nil))
	if response.Code != http.StatusConflict {
		t.Fatalf("conflicting approve=%d", response.Code)
	}
	response = httptest.NewRecorder()
	f.handler.ServeHTTP(response, authorizedRequest(http.MethodPost, "/verification-requests/"+id+"/complete", nil, ""))
	if response.Code != http.StatusConflict {
		t.Fatalf("complete denied=%d", response.Code)
	}
}

func TestCompletionReflectsSupersededAfterApproval(t *testing.T) {
	f := newConsentFixture(t, "OFFICIAL", "birth")
	hash := f.linkCredential(t, chain.DocBirthCertificate, true)
	id := createRequest(t, f)
	token := consentToken(t, f, id)
	response := httptest.NewRecorder()
	f.handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/consent/"+token+"/approve", nil))
	record := f.registry.records[hash]
	record.Status = chain.StatusSuperseded
	f.registry.records[hash] = record
	newHash := sha256.Sum256([]byte("replacement"))
	f.registry.records[newHash] = chain.Record{Found: true, DocID: "DOC-2", DocType: chain.DocBirthCertificate, Status: chain.StatusValid}
	f.registry.current["DOC-1"] = newHash
	response = httptest.NewRecorder()
	f.handler.ServeHTTP(response, authorizedRequest(http.MethodPost, "/verification-requests/"+id+"/complete", nil, ""))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"status":"SUPERSEDED"`) {
		t.Fatalf("superseded completion=%d %s", response.Code, response.Body.String())
	}
}

func TestConsentRequestRejectsUnlinkedAndCrossDepartment(t *testing.T) {
	f := newConsentFixture(t, "OFFICIAL", "birth")
	f.linkCredential(t, chain.DocBirthCertificate, false)
	response := httptest.NewRecorder()
	f.handler.ServeHTTP(response, authorizedRequest(http.MethodPost, "/verification-requests", bytes.NewBufferString(`{"documentId":"DOC-1","purpose":"test"}`), "application/json"))
	if response.Code != http.StatusConflict {
		t.Fatalf("unlinked=%d %s", response.Code, response.Body.String())
	}
	f2 := newConsentFixture(t, "OFFICIAL", "birth")
	f2.linkCredential(t, chain.DocDrivingLicence, true)
	response = httptest.NewRecorder()
	f2.handler.ServeHTTP(response, authorizedRequest(http.MethodPost, "/verification-requests", bytes.NewBufferString(`{"documentId":"DOC-1","purpose":"test"}`), "application/json"))
	if response.Code != http.StatusForbidden {
		t.Fatalf("cross department=%d", response.Code)
	}
}

func TestControllerCannotCreateVerificationRequest(t *testing.T) {
	f := newConsentFixture(t, "CONTROLLER", "")
	response := httptest.NewRecorder()
	f.handler.ServeHTTP(response, authorizedRequest(http.MethodPost, "/verification-requests", bytes.NewBufferString(`{}`), "application/json"))
	if response.Code != http.StatusForbidden {
		t.Fatalf("controller=%d", response.Code)
	}
}

func TestNotificationFailureIsRecorded(t *testing.T) {
	f := newConsentFixture(t, "OFFICIAL", "birth")
	f.linkCredential(t, chain.DocBirthCertificate, true)
	f.notifications.err = errors.New("delivery unavailable")
	response := httptest.NewRecorder()
	f.handler.ServeHTTP(response, authorizedRequest(http.MethodPost, "/verification-requests", bytes.NewBufferString(`{"documentId":"DOC-1","purpose":"test"}`), "application/json"))
	if response.Code != http.StatusBadGateway {
		t.Fatalf("notification failure=%d %s", response.Code, response.Body.String())
	}
	page, err := f.store.AuditEvents(store.AuditQuery{Action: actionNotificationAttempt, Limit: 10})
	if err != nil || len(page.Events) != 1 || page.Events[0].Outcome != "FAILURE" {
		t.Fatalf("notification audit=%#v err=%v", page.Events, err)
	}
}

func TestLinkedDirectIDVerificationRequiresConsent(t *testing.T) {
	f := newConsentFixture(t, "OFFICIAL", "birth")
	f.linkCredential(t, chain.DocBirthCertificate, true)
	response := httptest.NewRecorder()
	f.handler.ServeHTTP(response, authorizedRequest(http.MethodGet, "/verify/DOC-1", nil, ""))
	if response.Code != http.StatusConflict {
		t.Fatalf("direct linked verify=%d %s", response.Code, response.Body.String())
	}
}
