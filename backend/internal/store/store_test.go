package store

import (
	"path/filepath"
	"testing"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestDefaultDepartmentsAreStableAndRenameSurvivesOpen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}

	departments, err := s.Departments()
	if err != nil {
		t.Fatal(err)
	}
	if len(departments) != 3 {
		t.Fatalf("got %d departments, want 3", len(departments))
	}
	wantTypes := map[string]uint8{"birth": 1, "transport": 2, "education": 3}
	for _, d := range departments {
		if d.DocType != wantTypes[d.ID] {
			t.Errorf("department %s type = %d, want %d", d.ID, d.DocType, wantTypes[d.ID])
		}
	}
	if _, err := s.db.Exec(`UPDATE departments SET display_name = 'Civil Registration' WHERE id = 'birth'`); err != nil {
		t.Fatal(err)
	}
	s.Close()

	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	birth, err := s.DepartmentByID("birth")
	if err != nil {
		t.Fatal(err)
	}
	if birth.DisplayName != "Civil Registration" {
		t.Fatalf("display name was overwritten: %q", birth.DisplayName)
	}
}

func TestUserProfileConstraintsAndLookup(t *testing.T) {
	s := openTestStore(t)

	valid := UserProfile{
		SupabaseUserID: "official-1", Email: "official@example.gov",
		DisplayName: "Birth Official", Role: "OFFICIAL", Active: true,
	}
	if err := s.UpsertUserProfile(valid, "birth"); err != nil {
		t.Fatal(err)
	}
	got, err := s.UserProfileByID(valid.SupabaseUserID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Department == nil || got.Department.ID != "birth" || got.Department.DocType != 1 {
		t.Fatalf("profile department = %#v", got.Department)
	}

	tests := []struct {
		name       string
		profile    UserProfile
		department string
	}{
		{"invalid role", UserProfile{SupabaseUserID: "bad-role", Email: "x", Role: "ROOT", Active: true}, "birth"},
		{"controller with department", UserProfile{SupabaseUserID: "controller-dept", Email: "x", Role: "CONTROLLER", Active: true}, "birth"},
		{"admin without department", UserProfile{SupabaseUserID: "admin-none", Email: "x", Role: "ADMIN", Active: true}, ""},
		{"official unknown department", UserProfile{SupabaseUserID: "official-unknown", Email: "x", Role: "OFFICIAL", Active: true}, "unknown"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := s.UpsertUserProfile(tc.profile, tc.department); err == nil {
				t.Fatal("expected database constraint error")
			}
		})
	}

	if _, err := s.db.Exec(`INSERT INTO user_profiles (supabase_user_id, email, role, active) VALUES ('bad-active', 'x', 'CONTROLLER', 2)`); err == nil {
		t.Fatal("expected active constraint error")
	}
}
