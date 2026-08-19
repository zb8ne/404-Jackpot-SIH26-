package store

import (
	"errors"
	"testing"
	"time"
)

func TestCitizenAccountRequiresEmailAndDocumentLink(t *testing.T) {
	s := openTestStore(t)
	if err := s.UpsertCitizenAccount(CitizenAccount{ID: "citizen-1", DisplayName: "Citizen", Active: true}); err == nil {
		t.Fatal("expected mandatory email error")
	}
	account := CitizenAccount{ID: "citizen-1", DisplayName: "Citizen", Email: "citizen@example.test", Active: true}
	if err := s.UpsertCitizenAccount(account); err != nil {
		t.Fatal(err)
	}
	got, err := s.CitizenAccountByID(account.ID)
	if err != nil || got.Email != account.Email {
		t.Fatalf("account = %#v, err=%v", got, err)
	}
	id := account.ID
	if err := s.Save(Document{DocHash: "0x01", DocID: "DOC-1", DocType: "birth_certificate", Citizen: "Citizen", Issuer: "issuer", Filename: "x.pdf", TxHash: "tx", CitizenAccountID: &id}, []byte("pdf")); err != nil {
		t.Fatal(err)
	}
	doc, found, err := s.ByDocumentID("DOC-1")
	if err != nil || !found || doc.CitizenAccountID == nil || *doc.CitizenAccountID != id {
		t.Fatalf("document=%#v found=%v err=%v", doc, found, err)
	}
}

func TestCitizenAccountSupabaseIdentityIsUniqueAndResolvable(t *testing.T) {
	s := openTestStore(t)
	supabaseID := "supabase-citizen-1"
	account := CitizenAccount{
		ID: "citizen-1", SupabaseUserID: &supabaseID,
		DisplayName: "Citizen", Email: "citizen@example.test", Active: true,
	}
	if err := s.UpsertCitizenAccount(account); err != nil {
		t.Fatal(err)
	}
	got, err := s.CitizenAccountBySupabaseUserID(supabaseID)
	if err != nil || got.ID != account.ID {
		t.Fatalf("account=%#v err=%v", got, err)
	}

	// Routine provisioning without the optional flag must not silently unlink
	// an identity that was linked previously.
	account.SupabaseUserID = nil
	account.DisplayName = "Updated Citizen"
	if err := s.UpsertCitizenAccount(account); err != nil {
		t.Fatal(err)
	}
	got, err = s.CitizenAccountBySupabaseUserID(supabaseID)
	if err != nil || got.DisplayName != account.DisplayName {
		t.Fatalf("preserved account=%#v err=%v", got, err)
	}

	duplicate := CitizenAccount{
		ID: "citizen-2", SupabaseUserID: &supabaseID,
		DisplayName: "Other Citizen", Email: "other@example.test", Active: true,
	}
	if err := s.UpsertCitizenAccount(duplicate); err == nil {
		t.Fatal("expected duplicate Supabase identity to be rejected")
	}
	if _, err := s.CitizenAccountBySupabaseUserID("unknown"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown identity error=%v, want ErrNotFound", err)
	}
}

func TestConsentDecisionIsAtomicAndSinglePurpose(t *testing.T) {
	s := openTestStore(t)
	seedVerificationRequest(t, s, time.Now().UTC().Add(time.Hour))
	event := AuditEvent{Actor: AuditActor{ID: "citizen:citizen-1", Role: "CITIZEN_CONSENT"}, Action: "CONSENT_APPROVED", Outcome: "SUCCESS", Result: "APPROVED", HTTPStatus: 200}
	req, err := s.DecideConsent("token-hash", "APPROVED", event)
	if err != nil || req.State != "APPROVED" {
		t.Fatalf("approve=%#v err=%v", req, err)
	}
	if _, err := s.DecideConsent("token-hash", "APPROVED", event); err != nil {
		t.Fatalf("identical decision should be idempotent: %v", err)
	}
	if _, err := s.DecideConsent("token-hash", "DENIED", event); !errors.Is(err, ErrConflict) {
		t.Fatalf("conflicting decision error=%v", err)
	}
}

func TestConsentRollsBackWhenAuditFails(t *testing.T) {
	s := openTestStore(t)
	seedVerificationRequest(t, s, time.Now().UTC().Add(time.Hour))
	if _, err := s.db.Exec(`CREATE TRIGGER fail_consent_audit BEFORE INSERT ON audit_logs WHEN NEW.action='CONSENT_APPROVED' BEGIN SELECT RAISE(FAIL,'audit failed'); END`); err != nil {
		t.Fatal(err)
	}
	event := AuditEvent{Actor: AuditActor{ID: "citizen:citizen-1", Role: "CITIZEN_CONSENT"}, Action: "CONSENT_APPROVED", Outcome: "SUCCESS", HTTPStatus: 200}
	if _, err := s.DecideConsent("token-hash", "APPROVED", event); err == nil {
		t.Fatal("expected audit failure")
	}
	req, err := s.VerificationRequestByID("request-1")
	if err != nil || req.State != "PENDING" {
		t.Fatalf("transition was not rolled back: %#v %v", req, err)
	}
}

func TestExpiredConsentAndDoubleCompletion(t *testing.T) {
	s := openTestStore(t)
	seedVerificationRequest(t, s, time.Now().UTC().Add(-time.Minute))
	event := AuditEvent{Actor: AuditActor{ID: "citizen:citizen-1", Role: "CITIZEN_CONSENT"}, Action: "CONSENT_APPROVED", Outcome: "SUCCESS", HTTPStatus: 200}
	if _, err := s.DecideConsent("token-hash", "APPROVED", event); !errors.Is(err, ErrExpired) {
		t.Fatalf("expired error=%v", err)
	}
	req, _ := s.VerificationRequestByID("request-1")
	if req.State != "EXPIRED" {
		t.Fatalf("state=%s", req.State)
	}

	s2 := openTestStore(t)
	seedVerificationRequest(t, s2, time.Now().UTC().Add(time.Hour))
	if _, err := s2.DecideConsent("token-hash", "APPROVED", event); err != nil {
		t.Fatal(err)
	}
	complete := AuditEvent{Actor: AuditActor{ID: "official", Role: "OFFICIAL"}, Action: "VERIFICATION_REQUEST_COMPLETED", Outcome: "SUCCESS", HTTPStatus: 200}
	if _, err := s2.CompleteVerificationRequest("request-1", map[string]any{"status": "VALID"}, complete); err != nil {
		t.Fatal(err)
	}
	if _, err := s2.CompleteVerificationRequest("request-1", map[string]any{"status": "VALID"}, complete); !errors.Is(err, ErrConflict) {
		t.Fatalf("double completion error=%v", err)
	}
}

func TestApprovedRequestCannotCompleteAfterExpiry(t *testing.T) {
	s := openTestStore(t)
	seedVerificationRequest(t, s, time.Now().UTC().Add(time.Hour))
	event := AuditEvent{Actor: AuditActor{ID: "citizen:citizen-1", Role: "CITIZEN_CONSENT"}, Action: "CONSENT_APPROVED", Outcome: "SUCCESS", HTTPStatus: 200}
	if _, err := s.DecideConsent("token-hash", "APPROVED", event); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`UPDATE verification_requests SET expires_at=? WHERE id='request-1'`, time.Now().UTC().Add(-time.Minute).Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	complete := AuditEvent{Actor: AuditActor{ID: "official", Role: "OFFICIAL"}, Action: "VERIFICATION_REQUEST_COMPLETED", Outcome: "SUCCESS", HTTPStatus: 200}
	if _, err := s.CompleteVerificationRequest("request-1", map[string]any{"status": "VALID"}, complete); !errors.Is(err, ErrConflict) {
		t.Fatalf("completion error = %v, want ErrConflict", err)
	}
}

func seedVerificationRequest(t *testing.T, s *Store, expires time.Time) {
	t.Helper()
	if err := s.UpsertCitizenAccount(CitizenAccount{ID: "citizen-1", DisplayName: "Citizen", Email: "citizen@example.test", Active: true}); err != nil {
		t.Fatal(err)
	}
	req := VerificationRequest{ID: "request-1", DocumentID: "DOC-1", ReferenceHash: "0x01", CitizenAccountID: "citizen-1", RequesterUserID: "official", RequesterRole: "OFFICIAL", DepartmentID: "birth", DepartmentName: "Birth", DocumentType: "birth_certificate", Purpose: "test", CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), ExpiresAt: expires.Format(time.RFC3339Nano)}
	token := "token-hash"
	req.ApprovalTokenHash = &token
	event := AuditEvent{Actor: AuditActor{ID: "official", Role: "OFFICIAL"}, Action: "VERIFICATION_REQUEST_CREATED", Outcome: "SUCCESS", HTTPStatus: 201}
	if err := s.CreateVerificationRequest(req, event); err != nil {
		t.Fatal(err)
	}
}
