package store

import "testing"

func TestAuditChainAppendQueryAndIntegrity(t *testing.T) {
	s := openTestStore(t)
	department := &AuditDepartment{ID: "birth", Name: "Birth Registration Dept"}
	for _, action := range []string{"ISSUE", "VERIFY_FILE"} {
		if _, err := s.AppendAuditEvent(AuditEvent{
			Actor:      AuditActor{ID: "user-1", Email: "user@example.gov", Role: "OFFICIAL"},
			Department: department, Action: action, Outcome: "SUCCESS", Result: "VALID",
			HTTPStatus: 200, Details: map[string]any{"key": "value"},
		}); err != nil {
			t.Fatal(err)
		}
	}
	page, err := s.AuditEvents(AuditQuery{Limit: 1, DepartmentID: "birth"})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Events) != 1 || !page.HasMore || page.NextBefore == 0 {
		t.Fatalf("unexpected page: %#v", page)
	}
	if page.Events[0].Action != "VERIFY_FILE" || page.Events[0].PreviousHash == zeroAuditHash {
		t.Fatalf("unexpected newest event: %#v", page.Events[0])
	}
	integrity, err := s.VerifyAuditChain()
	if err != nil {
		t.Fatal(err)
	}
	if !integrity.Valid || integrity.EventCount != 2 {
		t.Fatalf("unexpected integrity: %#v", integrity)
	}
}

func TestAuditIntegrityDetectsMutation(t *testing.T) {
	s := openTestStore(t)
	event, err := s.AppendAuditEvent(AuditEvent{
		Actor: AuditActor{ID: "user-1", Role: "ADMIN"}, Action: "REVOKE",
		Outcome: "SUCCESS", Result: "REVOKED", HTTPStatus: 200,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`UPDATE audit_logs SET result = 'VALID' WHERE id = ?`, event.ID); err != nil {
		t.Fatal(err)
	}
	integrity, err := s.VerifyAuditChain()
	if err != nil {
		t.Fatal(err)
	}
	if integrity.Valid || integrity.FirstInvalidEventID == nil || *integrity.FirstInvalidEventID != event.ID {
		t.Fatalf("mutation was not detected: %#v", integrity)
	}
}

func TestAuditedProfileChangeCapturesBeforeAndAfter(t *testing.T) {
	s := openTestStore(t)
	profile := UserProfile{SupabaseUserID: "admin-1", Email: "a@example.gov", Role: "ADMIN", Active: true}
	actor := AuditActor{ID: "operator", Role: "SYSTEM"}
	if err := s.UpsertUserProfileAudited(profile, "birth", actor); err != nil {
		t.Fatal(err)
	}
	profile.Role = "OFFICIAL"
	if err := s.UpsertUserProfileAudited(profile, "birth", actor); err != nil {
		t.Fatal(err)
	}
	page, err := s.AuditEvents(AuditQuery{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Events) != 2 || page.Events[0].Action != "USER_PROFILE_UPDATE" || page.Events[1].Action != "USER_PROFILE_CREATE" {
		t.Fatalf("unexpected profile audit events: %#v", page.Events)
	}
	before := page.Events[0].Details["before"].(map[string]any)
	if before["role"] != "ADMIN" {
		t.Fatalf("before role = %v", before["role"])
	}
}

func TestProfileChangeRollsBackWhenAuditInsertFails(t *testing.T) {
	s := openTestStore(t)
	if _, err := s.db.Exec(`
		CREATE TRIGGER fail_profile_audit
		BEFORE INSERT ON audit_logs
		WHEN NEW.action IN ('USER_PROFILE_CREATE', 'USER_PROFILE_UPDATE')
		BEGIN
		  SELECT RAISE(FAIL, 'simulated audit failure');
		END`); err != nil {
		t.Fatal(err)
	}
	profile := UserProfile{
		SupabaseUserID: "must-rollback", Email: "rollback@example.gov",
		Role: "ADMIN", Active: true,
	}
	err := s.UpsertUserProfileAudited(profile, "birth", AuditActor{ID: "operator", Role: "SYSTEM"})
	if err == nil {
		t.Fatal("expected audit insertion failure")
	}
	if _, err := s.UserProfileByID(profile.SupabaseUserID); err != ErrNotFound {
		t.Fatalf("profile change survived failed audit insert: %v", err)
	}
	page, err := s.AuditEvents(AuditQuery{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Events) != 0 {
		t.Fatalf("unexpected audit rows after rollback: %#v", page.Events)
	}
}
