package rbac

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"credreg/backend/internal/auth"
	"credreg/backend/internal/store"
)

func TestPermissionMatrix(t *testing.T) {
	all := []Permission{
		PermissionIssue, PermissionVerify, PermissionSupersede,
		PermissionRevoke, PermissionViewDept, PermissionMonitorAll,
	}
	want := map[Role]map[Permission]bool{
		RoleController: {PermissionMonitorAll: true},
		RoleAdmin: {
			PermissionIssue: true, PermissionVerify: true, PermissionSupersede: true,
			PermissionRevoke: true, PermissionViewDept: true,
		},
		RoleOfficial: {
			PermissionIssue: true, PermissionVerify: true, PermissionViewDept: true,
		},
	}
	for role, permissions := range want {
		for _, permission := range all {
			if got := Allowed(role, permission); got != permissions[permission] {
				t.Errorf("Allowed(%s, %s) = %v, want %v", role, permission, got, permissions[permission])
			}
		}
	}
}

type fakeVerifier struct{ user *auth.User }

func (f fakeVerifier) Verify(context.Context, string) (*auth.User, error) {
	if f.user == nil {
		return nil, errors.New("invalid")
	}
	return f.user, nil
}

type fakeProfiles struct {
	profile store.UserProfile
	err     error
}

func (f fakeProfiles) UserProfileByID(string) (store.UserProfile, error) {
	return f.profile, f.err
}

func TestAuthenticationProfileAndPermissionFailures(t *testing.T) {
	official := store.UserProfile{
		SupabaseUserID: "official", Role: "OFFICIAL", Active: true,
		Department: &store.Department{ID: "birth", Active: true},
	}
	tests := []struct {
		name       string
		verifier   auth.TokenVerifier
		profiles   profileStore
		permission Permission
		header     bool
		want       int
	}{
		{"missing token", fakeVerifier{user: &auth.User{ID: "official"}}, fakeProfiles{profile: official}, PermissionIssue, false, http.StatusUnauthorized},
		{"invalid token", fakeVerifier{}, fakeProfiles{profile: official}, PermissionIssue, true, http.StatusUnauthorized},
		{"unknown profile", fakeVerifier{user: &auth.User{ID: "unknown"}}, fakeProfiles{err: store.ErrNotFound}, PermissionIssue, true, http.StatusForbidden},
		{"inactive profile", fakeVerifier{user: &auth.User{ID: "official"}}, fakeProfiles{profile: store.UserProfile{Role: "OFFICIAL"}}, PermissionIssue, true, http.StatusForbidden},
		{"forbidden permission", fakeVerifier{user: &auth.User{ID: "official"}}, fakeProfiles{profile: official}, PermissionRevoke, true, http.StatusForbidden},
		{"allowed", fakeVerifier{user: &auth.User{ID: "official"}}, fakeProfiles{profile: official}, PermissionIssue, true, http.StatusNoContent},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			final := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if _, ok := PrincipalFromContext(r.Context()); !ok {
					t.Error("principal missing")
				}
				w.WriteHeader(http.StatusNoContent)
			})
			handler := auth.Middleware(tc.verifier)(LoadProfile(tc.profiles)(Require(tc.permission)(final)))
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.header {
				req.Header.Set("Authorization", "Bearer token")
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, req)
			if response.Code != tc.want {
				t.Fatalf("status = %d, want %d", response.Code, tc.want)
			}
		})
	}
}
