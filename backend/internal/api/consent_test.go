package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	handler := newHandlerConfigured(registry, s, apiVerifier{user: &auth.User{ID: "user-1"}}, "0xcontract", "http://web.test", "http://api.test", notifications)
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

func TestConsentRequestIsDeliveredToCitizenInboxFor24Hours(t *testing.T) {
	f := newConsentFixture(t, "OFFICIAL", "birth")
	f.linkCredential(t, chain.DocBirthCertificate, true)
	id := createRequest(t, f)
	req, err := f.store.VerificationRequestByID(id)
	if err != nil {
		t.Fatal(err)
	}
	if req.ApprovalTokenHash != nil {
		t.Fatal("new inbox request unexpectedly created an email-link token")
	}
	createdAt, _ := time.Parse(time.RFC3339Nano, req.CreatedAt)
	expiresAt, _ := time.Parse(time.RFC3339Nano, req.ExpiresAt)
	if duration := expiresAt.Sub(createdAt); duration != 24*time.Hour {
		t.Fatalf("request lifetime=%s, want 24h", duration)
	}
	citizenID := "citizen-user"
	if err := f.store.UpsertCitizenAccount(store.CitizenAccount{ID: "citizen-1", SupabaseUserID: &citizenID, DisplayName: "Citizen", Email: "citizen@example.test", Active: true}); err != nil {
		t.Fatal(err)
	}
	citizenHandler := newHandlerConfigured(f.registry, f.store, apiVerifier{user: &auth.User{ID: citizenID}}, "0xcontract", "http://web.test", "http://api.test", f.notifications)
	response := httptest.NewRecorder()
	citizenHandler.ServeHTTP(response, authorizedRequest(http.MethodGet, "/citizen/verification-requests", nil, ""))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), id) || !strings.Contains(response.Body.String(), `"notificationStatus":"SUCCEEDED"`) {
		t.Fatalf("inbox status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestCitizenInboxDecisionPersistsAcrossSessionsAndOfficialCompletes(t *testing.T) {
	f := newConsentFixture(t, "OFFICIAL", "birth")
	f.linkCredential(t, chain.DocBirthCertificate, true)
	id := createRequest(t, f)
	citizenID := "citizen-user"
	if err := f.store.UpsertCitizenAccount(store.CitizenAccount{ID: "citizen-1", SupabaseUserID: &citizenID, DisplayName: "Citizen", Email: "citizen@example.test", Active: true}); err != nil {
		t.Fatal(err)
	}
	newCitizenSession := func() http.Handler {
		return newHandlerConfigured(f.registry, f.store, apiVerifier{user: &auth.User{ID: citizenID}}, "0xcontract", "http://web.test", "http://api.test", f.notifications)
	}
	response := httptest.NewRecorder()
	newCitizenSession().ServeHTTP(response, authorizedRequest(http.MethodPost, "/citizen/verification-requests/"+id+"/decision", bytes.NewBufferString(`{"decision":"APPROVED"}`), "application/json"))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"state":"APPROVED"`) {
		t.Fatalf("approve status=%d body=%s", response.Code, response.Body.String())
	}
	response = httptest.NewRecorder()
	newCitizenSession().ServeHTTP(response, authorizedRequest(http.MethodGet, "/citizen/verification-requests", nil, ""))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"state":"APPROVED"`) {
		t.Fatalf("persisted inbox status=%d body=%s", response.Code, response.Body.String())
	}
	response = httptest.NewRecorder()
	f.handler.ServeHTTP(response, authorizedRequest(http.MethodPost, "/verification-requests/"+id+"/complete", nil, ""))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"status":"VALID"`) {
		t.Fatalf("complete status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestCitizenCannotDecideAnotherCitizensRequest(t *testing.T) {
	f := newConsentFixture(t, "OFFICIAL", "birth")
	f.linkCredential(t, chain.DocBirthCertificate, true)
	id := createRequest(t, f)
	otherID := "other-user"
	if err := f.store.UpsertCitizenAccount(store.CitizenAccount{ID: "citizen-2", SupabaseUserID: &otherID, DisplayName: "Other", Email: "other@example.test", Active: true}); err != nil {
		t.Fatal(err)
	}
	handler := newHandlerConfigured(f.registry, f.store, apiVerifier{user: &auth.User{ID: otherID}}, "0xcontract", "http://web.test", "http://api.test", f.notifications)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authorizedRequest(http.MethodPost, "/citizen/verification-requests/"+id+"/decision", bytes.NewBufferString(`{"decision":"DENIED"}`), "application/json"))
	if response.Code != http.StatusNotFound {
		t.Fatalf("cross-citizen decision status=%d, want 404", response.Code)
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

func TestLinkedDirectIDVerificationRequiresConsent(t *testing.T) {
	f := newConsentFixture(t, "OFFICIAL", "birth")
	f.linkCredential(t, chain.DocBirthCertificate, true)
	response := httptest.NewRecorder()
	f.handler.ServeHTTP(response, authorizedRequest(http.MethodGet, "/verify/DOC-1", nil, ""))
	if response.Code != http.StatusConflict {
		t.Fatalf("direct linked verify=%d %s", response.Code, response.Body.String())
	}
}

func TestQRPNGDownloadIsPublicForIssuedCredential(t *testing.T) {
	f := newConsentFixture(t, "OFFICIAL", "birth")
	f.linkCredential(t, chain.DocBirthCertificate, true)
	response := httptest.NewRecorder()
	f.handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/qr/DOC-1/download.png", nil))
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "image/png" || !bytes.HasPrefix(response.Body.Bytes(), []byte("\x89PNG\r\n\x1a\n")) {
		t.Fatalf("QR download status=%d type=%q body-prefix=%q", response.Code, response.Header().Get("Content-Type"), response.Body.Bytes()[:min(8, response.Body.Len())])
	}
	if !strings.Contains(response.Header().Get("Content-Disposition"), "DOC-1-qr.png") {
		t.Fatalf("content disposition=%q", response.Header().Get("Content-Disposition"))
	}
}
