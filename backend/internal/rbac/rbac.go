package rbac

import (
	"context"
	"errors"
	"net/http"

	"credreg/backend/internal/auth"
	"credreg/backend/internal/store"
)

type Role string

const (
	RoleController Role = "CONTROLLER"
	RoleAdmin      Role = "ADMIN"
	RoleOfficial   Role = "OFFICIAL"
)

type Permission string

const (
	PermissionIssue               Permission = "issue"
	PermissionVerify              Permission = "verify"
	PermissionSupersede           Permission = "supersede"
	PermissionRevoke              Permission = "revoke"
	PermissionViewDept            Permission = "view_department"
	PermissionViewAudit           Permission = "view_department_audit"
	PermissionRequestVerification Permission = "request_verification"
	PermissionMonitorAll          Permission = "monitor_all"
)

var permissions = map[Role]map[Permission]bool{
	RoleController: {PermissionMonitorAll: true},
	RoleAdmin: {
		PermissionIssue: true, PermissionVerify: true, PermissionSupersede: true,
		PermissionRevoke: true, PermissionViewDept: true,
		PermissionViewAudit:           true,
		PermissionRequestVerification: true,
	},
	RoleOfficial: {
		PermissionIssue: true, PermissionVerify: true, PermissionViewDept: true,
		PermissionRequestVerification: true,
	},
}

func Allowed(role Role, permission Permission) bool {
	return permissions[role][permission]
}

type Principal struct {
	Identity *auth.User
	Profile  store.UserProfile
}

type profileStore interface {
	UserProfileByID(string) (store.UserProfile, error)
}

type contextKey string

const principalContextKey contextKey = "application-principal"

func PrincipalFromContext(ctx context.Context) (*Principal, bool) {
	p, ok := ctx.Value(principalContextKey).(*Principal)
	return p, ok
}

// LoadProfile turns a verified Supabase identity into the backend-owned
// application identity used for every authorization decision.
func LoadProfile(profiles profileStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			identity, ok := auth.UserFromContext(r.Context())
			if !ok {
				http.Error(w, "authenticated user missing from context", http.StatusUnauthorized)
				return
			}

			profile, err := profiles.UserProfileByID(identity.ID)
			if errors.Is(err, store.ErrNotFound) {
				http.Error(w, "authenticated user has no application profile", http.StatusForbidden)
				return
			}
			if err != nil {
				http.Error(w, "could not load application profile", http.StatusInternalServerError)
				return
			}
			if !profile.Active || profile.Department != nil && !profile.Department.Active {
				http.Error(w, "application profile is inactive", http.StatusForbidden)
				return
			}

			principal := &Principal{Identity: identity, Profile: profile}
			ctx := context.WithValue(r.Context(), principalContextKey, principal)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func Require(permission Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := PrincipalFromContext(r.Context())
			if !ok {
				http.Error(w, "application profile missing from context", http.StatusUnauthorized)
				return
			}
			if !Allowed(Role(principal.Profile.Role), permission) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
