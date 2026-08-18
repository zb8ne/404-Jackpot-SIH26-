package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"credreg/backend/internal/rbac"
	"credreg/backend/internal/store"
)

const (
	actionIssue       = "ISSUE"
	actionVerifyFile  = "VERIFY_FILE"
	actionVerifyID    = "VERIFY_ID"
	actionRevoke      = "REVOKE"
	actionSupersede   = "SUPERSEDE"
	maxAuditPageLimit = 100
)

type auditContextKey string

const auditOperationKey auditContextKey = "audit-operation"

type auditOperation struct {
	Result        string
	DocID         string
	DocHash       string
	Citizen       string
	Transaction   string
	ReferenceHash string
	Outcome       string
	Details       map[string]any
}

func operationFromContext(ctx context.Context) *auditOperation {
	op, _ := ctx.Value(auditOperationKey).(*auditOperation)
	return op
}

type auditResponseWriter struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (w *auditResponseWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *auditResponseWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	if w.body.Len() < 64<<10 {
		_, _ = w.body.Write(body)
	}
	return w.ResponseWriter.Write(body)
}

func (s *Server) audit(action string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := rbac.PrincipalFromContext(r.Context())
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		op := &auditOperation{Details: map[string]any{}}
		requestID := newOpaqueID()
		ctx := context.WithValue(r.Context(), auditOperationKey, op)
		recorder := &auditResponseWriter{ResponseWriter: w}
		next.ServeHTTP(recorder, r.WithContext(ctx))
		if recorder.status == 0 {
			recorder.status = http.StatusOK
		}

		outcome := op.Outcome
		if outcome == "" {
			switch {
			case recorder.status >= 200 && recorder.status < 300:
				outcome = "SUCCESS"
			case recorder.status == http.StatusForbidden:
				outcome = "DENIED"
			default:
				outcome = "FAILURE"
			}
		}
		event := store.AuditEvent{
			Actor: store.AuditActor{
				ID: principal.Profile.SupabaseUserID, Email: principal.Profile.Email,
				Name: principal.Profile.DisplayName, Role: principal.Profile.Role,
			},
			Action: action, Outcome: outcome, Result: op.Result, RequestID: requestID,
			HTTPStatus: recorder.status, Details: op.Details,
			Credential: store.AuditCredential{
				DocID: optionalString(op.DocID), DocHash: optionalString(op.DocHash),
				Citizen: optionalString(op.Citizen), TransactionHash: optionalString(op.Transaction),
				ReferenceHash: optionalString(op.ReferenceHash),
			},
		}
		if d := principal.Profile.Department; d != nil {
			event.Department = &store.AuditDepartment{ID: d.ID, Name: d.DisplayName}
		}
		if recorder.status >= 400 {
			message := responseError(recorder.body.Bytes())
			event.Error = &message
		}
		if _, err := s.store.AppendAuditEvent(event); err != nil {
			log.Printf("audit append failed for request %s action %s: %v", requestID, action, err)
		}
	})
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func responseError(raw []byte) string {
	var body struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(raw, &body) == nil && body.Error != "" {
		return body.Error
	}
	return strings.TrimSpace(string(raw))
}

func newOpaqueID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return strconv.FormatInt(time.Now().UTC().UnixNano(), 36)
	}
	return hex.EncodeToString(value[:])
}

func (s *Server) auditEventsAccess(w http.ResponseWriter, r *http.Request) {
	principal, ok := rbac.PrincipalFromContext(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "application profile missing from context")
		return
	}
	role := rbac.Role(principal.Profile.Role)
	if !rbac.Allowed(role, rbac.PermissionMonitorAll) && !rbac.Allowed(role, rbac.PermissionViewAudit) {
		writeErr(w, http.StatusForbidden, "forbidden")
		return
	}
	s.auditEvents(w, r)
}

func (s *Server) auditEvents(w http.ResponseWriter, r *http.Request) {
	principal, _ := rbac.PrincipalFromContext(r.Context())
	query := store.AuditQuery{Limit: 50}
	if raw := r.URL.Query().Get("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > maxAuditPageLimit {
			writeErr(w, http.StatusBadRequest, "invalid limit")
			return
		}
		query.Limit = limit
	}
	if raw := r.URL.Query().Get("before"); raw != "" {
		before, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || before < 1 {
			writeErr(w, http.StatusBadRequest, "invalid before cursor")
			return
		}
		query.Before = before
	}
	query.Action = strings.ToUpper(r.URL.Query().Get("action"))
	query.Outcome = strings.ToUpper(r.URL.Query().Get("outcome"))
	query.DocumentID = r.URL.Query().Get("documentId")
	query.ActorUserID = r.URL.Query().Get("actorUserId")
	query.DepartmentID = r.URL.Query().Get("department")
	if query.Action != "" && !validAuditAction(query.Action) {
		writeErr(w, http.StatusBadRequest, "invalid action")
		return
	}
	if query.Outcome != "" && !validAuditOutcome(query.Outcome) {
		writeErr(w, http.StatusBadRequest, "invalid outcome")
		return
	}
	if principal.Profile.Role == string(rbac.RoleAdmin) {
		own := principal.Profile.Department.ID
		if query.DepartmentID != "" && query.DepartmentID != own {
			writeErr(w, http.StatusForbidden, "cannot access another department's audit events")
			return
		}
		query.DepartmentID = own
	}
	page, err := s.store.AuditEvents(query)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not load audit events")
		return
	}
	var next any
	if page.HasMore {
		next = page.NextBefore
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"events": page.Events,
		"page":   map[string]any{"limit": query.Limit, "nextBefore": next, "hasMore": page.HasMore},
	})
}

type operationCounts struct {
	Total          int `json:"total"`
	Success        int `json:"success"`
	Failure        int `json:"failure"`
	Denied         int `json:"denied"`
	PartialFailure int `json:"partialFailure"`
}

type departmentCounts struct {
	ID, Name       string
	Active         bool
	Events         int
	Success        int
	Failure        int
	Denied         int
	PartialFailure int
	Issue          int
	Verify         int
	Revoke         int
	Supersede      int
	LastActivityAt *string
}

func (s *Server) monitoringOverview(w http.ResponseWriter, r *http.Request) {
	page, err := s.store.AuditEvents(store.AuditQuery{Limit: 1_000_000})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not load monitoring overview")
		return
	}
	integrity, err := s.store.VerifyAuditChain()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not load monitoring overview")
		return
	}
	departments, err := s.store.Departments()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not load monitoring overview")
		return
	}
	totals := operationCounts{}
	operations := map[string]*operationCounts{
		"issue": {}, "verify": {}, "revoke": {}, "supersede": {},
	}
	verdicts := map[string]int{
		"VALID": 0, "SUPERSEDED": 0, "REVOKED": 0, "TAMPERED": 0,
		"NOT_ISSUED": 0, "UNKNOWN": 0,
	}
	byDepartment := map[string]*departmentCounts{}
	for _, d := range departments {
		byDepartment[d.ID] = &departmentCounts{ID: d.ID, Name: d.DisplayName, Active: d.Active}
	}
	for _, event := range page.Events {
		addOutcome(&totals, event.Outcome)
		name := operationName(event.Action)
		if counts := operations[name]; counts != nil {
			addOutcome(counts, event.Outcome)
		}
		if strings.HasPrefix(event.Action, "VERIFY_") {
			if _, ok := verdicts[event.Result]; ok && event.Outcome == "SUCCESS" {
				verdicts[event.Result]++
			}
		}
		if event.Department != nil {
			counts := byDepartment[event.Department.ID]
			if counts == nil {
				counts = &departmentCounts{ID: event.Department.ID, Name: event.Department.Name}
				byDepartment[event.Department.ID] = counts
			}
			addDepartmentOutcome(counts, event.Outcome)
			switch name {
			case "issue":
				counts.Issue++
			case "verify":
				counts.Verify++
			case "revoke":
				counts.Revoke++
			case "supersede":
				counts.Supersede++
			}
			if counts.LastActivityAt == nil {
				created := event.CreatedAt
				counts.LastActivityAt = &created
			}
		}
	}
	recent := page.Events
	if len(recent) > 10 {
		recent = recent[:10]
	}
	departmentList := []map[string]any{}
	for _, d := range departments {
		counts := byDepartment[d.ID]
		departmentList = append(departmentList, map[string]any{
			"id": counts.ID, "name": counts.Name, "active": counts.Active,
			"events": counts.Events, "success": counts.Success, "failure": counts.Failure,
			"denied": counts.Denied, "partialFailure": counts.PartialFailure,
			"issue": counts.Issue, "verify": counts.Verify, "revoke": counts.Revoke,
			"supersede": counts.Supersede, "lastActivityAt": counts.LastActivityAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"generatedAt": time.Now().UTC().Format(time.RFC3339Nano), "auditIntegrity": integrity,
		"totals": totals, "operations": operations, "verificationResults": verdicts,
		"departments": departmentList, "recentActivity": recent,
	})
}

func addOutcome(counts *operationCounts, outcome string) {
	counts.Total++
	switch outcome {
	case "SUCCESS":
		counts.Success++
	case "FAILURE":
		counts.Failure++
	case "DENIED":
		counts.Denied++
	case "PARTIAL_FAILURE":
		counts.PartialFailure++
	}
}

func addDepartmentOutcome(counts *departmentCounts, outcome string) {
	counts.Events++
	switch outcome {
	case "SUCCESS":
		counts.Success++
	case "FAILURE":
		counts.Failure++
	case "DENIED":
		counts.Denied++
	case "PARTIAL_FAILURE":
		counts.PartialFailure++
	}
}

func operationName(action string) string {
	switch action {
	case actionIssue:
		return "issue"
	case actionVerifyFile, actionVerifyID:
		return "verify"
	case actionRevoke:
		return "revoke"
	case actionSupersede:
		return "supersede"
	}
	return ""
}

func validAuditAction(action string) bool {
	switch action {
	case actionIssue, actionVerifyFile, actionVerifyID, actionRevoke, actionSupersede,
		"USER_PROFILE_CREATE", "USER_PROFILE_UPDATE":
		return true
	}
	return false
}

func validAuditOutcome(outcome string) bool {
	switch outcome {
	case "SUCCESS", "FAILURE", "DENIED", "PARTIAL_FAILURE":
		return true
	}
	return false
}
