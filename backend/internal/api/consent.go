package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"credreg/backend/internal/chain"
	"credreg/backend/internal/rbac"
	"credreg/backend/internal/store"
)

const verificationRequestTTL = 15 * time.Minute

const (
	actionRequestCreated       = "VERIFICATION_REQUEST_CREATED"
	actionNotificationAttempt  = "CONSENT_NOTIFICATION"
	actionConsentApproved      = "CONSENT_APPROVED"
	actionConsentDenied        = "CONSENT_DENIED"
	actionRequestExpired       = "VERIFICATION_REQUEST_EXPIRED"
	actionRequestCompleted     = "VERIFICATION_REQUEST_COMPLETED"
	actionConsentTokenRejected = "CONSENT_TOKEN_REJECTED"
)

// developmentNotifier intentionally retains consent URLs only in process
// memory. Persistent notification rows and audit records contain a masked
// destination, never the raw token or URL.
type developmentNotifier struct {
	mu   sync.RWMutex
	urls map[string]string
	err  error
}

type notifier interface {
	Send(requestID, destination, consentURL string) error
}
type developmentCapture interface {
	URL(requestID string) (string, bool)
}

func newDevelopmentNotifier() *developmentNotifier {
	return &developmentNotifier{urls: map[string]string{}}
}

func (n *developmentNotifier) Send(requestID, _ string, consentURL string) error {
	if n.err != nil {
		return n.err
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	n.urls[requestID] = consentURL
	return nil
}

func (n *developmentNotifier) URL(requestID string) (string, bool) {
	n.mu.RLock()
	defer n.mu.RUnlock()
	value, ok := n.urls[requestID]
	return value, ok
}

func (s *Server) createVerificationRequest(w http.ResponseWriter, r *http.Request) {
	principal, _ := rbac.PrincipalFromContext(r.Context())
	if principal.Profile.Department == nil {
		writeErr(w, http.StatusForbidden, "forbidden")
		return
	}
	var body struct {
		DocumentID string `json:"documentId"`
		Purpose    string `json:"purpose"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "expected documentId and purpose")
		return
	}
	body.DocumentID, body.Purpose = strings.TrimSpace(body.DocumentID), strings.TrimSpace(body.Purpose)
	if body.DocumentID == "" || body.Purpose == "" {
		writeErr(w, http.StatusBadRequest, "documentId and purpose are required")
		return
	}
	if len(body.Purpose) > 500 {
		writeErr(w, http.StatusBadRequest, "purpose is too long")
		return
	}

	currentHash, known, err := s.reader.CurrentHashOf(r.Context(), body.DocumentID)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	if !known {
		writeErr(w, http.StatusNotFound, "credential not found")
		return
	}
	record, err := s.reader.Verify(r.Context(), currentHash)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	if !record.Found {
		writeErr(w, http.StatusNotFound, "credential not found")
		return
	}
	if record.DocType != principal.Profile.Department.DocType {
		writeErr(w, http.StatusForbidden, "credential is outside the authenticated department")
		return
	}
	hash := "0x" + hex.EncodeToString(currentHash[:])
	document, found, err := s.store.ByHash(hash)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not load credential linkage")
		return
	}
	if !found || document.CitizenAccountID == nil {
		writeErr(w, http.StatusConflict, "credential is not linked to a citizen account")
		return
	}
	account, err := s.store.CitizenAccountByID(*document.CitizenAccountID)
	if err != nil || !account.Active {
		writeErr(w, http.StatusConflict, "linked citizen account is unavailable")
		return
	}

	token, tokenHash, err := newConsentToken()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not create consent token")
		return
	}
	now := time.Now().UTC()
	request := store.VerificationRequest{
		ID: newOpaqueID(), DocumentID: body.DocumentID, ReferenceHash: hash,
		CitizenAccountID: account.ID, RequesterUserID: principal.Profile.SupabaseUserID,
		RequesterEmail: principal.Profile.Email, RequesterName: principal.Profile.DisplayName,
		RequesterRole: principal.Profile.Role, DepartmentID: principal.Profile.Department.ID,
		DepartmentName: principal.Profile.Department.DisplayName, DocumentType: chain.DocTypeName(record.DocType),
		State: "PENDING", Purpose: body.Purpose, CreatedAt: now.Format(time.RFC3339Nano),
		ExpiresAt: now.Add(verificationRequestTTL).Format(time.RFC3339Nano), ApprovalTokenHash: &tokenHash,
	}
	event := requestAuditEvent(principal, request, actionRequestCreated, "SUCCESS", "PENDING", http.StatusCreated)
	if err := s.store.CreateVerificationRequest(request, event); err != nil {
		writeErr(w, http.StatusInternalServerError, "could not create verification request")
		return
	}

	consentURL := s.publicWebURL + "/consent/" + token
	notifyErr := s.notifications.Send(request.ID, account.Email, consentURL)
	notifyStatus, auditOutcome := "SUCCEEDED", "SUCCESS"
	notifyError := ""
	if notifyErr != nil {
		notifyStatus, auditOutcome, notifyError = "FAILED", "FAILURE", notifyErr.Error()
	}
	notifyEvent := requestAuditEvent(principal, request, actionNotificationAttempt, auditOutcome, notifyStatus, http.StatusOK)
	if notifyError != "" {
		notifyEvent.Error = optionalString(notifyError)
	}
	if err := s.store.RecordNotification(request.ID, maskEmail(account.Email), notifyStatus, notifyError, notifyEvent); err != nil {
		writeErr(w, http.StatusInternalServerError, "request created but notification record failed")
		return
	}
	if notifyErr != nil {
		writeErr(w, http.StatusBadGateway, "verification request created but consent notification failed")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id": request.ID, "state": "PENDING", "expiresAt": request.ExpiresAt,
		"notification": map[string]any{"channel": "EMAIL", "destination": maskEmail(account.Email), "status": "SUCCEEDED"},
	})
}

func (s *Server) verificationRequest(w http.ResponseWriter, r *http.Request) {
	req, ok := s.authorizedVerificationRequest(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, req)
}

func (s *Server) verificationRequests(w http.ResponseWriter, r *http.Request) {
	principal, _ := rbac.PrincipalFromContext(r.Context())
	state := strings.ToUpper(r.URL.Query().Get("state"))
	if state != "" && !validRequestState(state) {
		writeErr(w, http.StatusBadRequest, "invalid state")
		return
	}
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 100 {
			writeErr(w, http.StatusBadRequest, "invalid limit")
			return
		}
		limit = value
	}
	offset := 0
	if raw := r.URL.Query().Get("offset"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 0 {
			writeErr(w, http.StatusBadRequest, "invalid offset")
			return
		}
		offset = value
	}
	query := store.VerificationRequestQuery{Limit: limit, Offset: offset, State: state, DepartmentID: principal.Profile.Department.ID}
	if principal.Profile.Role == string(rbac.RoleOfficial) {
		query.RequesterUserID = principal.Profile.SupabaseUserID
	}
	requests, err := s.store.VerificationRequests(query)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not load verification requests")
		return
	}
	for i := range requests {
		s.expireIfNeeded(&requests[i])
	}
	writeJSON(w, http.StatusOK, map[string]any{"requests": requests, "page": map[string]any{"limit": limit, "offset": offset, "hasMore": len(requests) == limit}})
}

func (s *Server) completeVerificationRequest(w http.ResponseWriter, r *http.Request) {
	principal, _ := rbac.PrincipalFromContext(r.Context())
	req, ok := s.authorizedVerificationRequest(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	if req.State != "APPROVED" {
		writeErr(w, http.StatusConflict, "verification request is not approved")
		return
	}
	expires, err := time.Parse(time.RFC3339Nano, req.ExpiresAt)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "verification request has an invalid expiry")
		return
	}
	if !time.Now().UTC().Before(expires) {
		writeErr(w, http.StatusConflict, "verification request approval has expired")
		return
	}
	referenceHash, err := parseHash(req.ReferenceHash)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "verification request has an invalid credential reference")
		return
	}
	record, err := s.reader.Verify(r.Context(), referenceHash)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	if !record.Found {
		writeErr(w, http.StatusNotFound, "credential is no longer available")
		return
	}
	resp := map[string]any{"docId": record.DocID, "expectedHash": req.ReferenceHash}
	s.describe(r.Context(), record, req.ReferenceHash, resp)
	if err := s.status(r.Context(), record, resp); err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	if !authorizeVerificationResponse(w, r, resp) {
		return
	}
	event := requestAuditEvent(principal, req, actionRequestCompleted, "SUCCESS", fmt.Sprint(resp["status"]), http.StatusOK)
	completed, err := s.store.CompleteVerificationRequest(req.ID, resp, event)
	if errors.Is(err, store.ErrConflict) {
		writeErr(w, http.StatusConflict, "verification request was already consumed or is no longer approved")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not complete verification request")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"request": completed, "verification": resp})
}

func (s *Server) developmentNotification(w http.ResponseWriter, r *http.Request) {
	req, ok := s.authorizedVerificationRequest(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	capture, ok := s.notifications.(developmentCapture)
	if !ok {
		writeErr(w, http.StatusNotFound, "development notification capture is disabled")
		return
	}
	value, found := capture.URL(req.ID)
	if !found {
		writeErr(w, http.StatusNotFound, "development consent URL is unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"requestId": req.ID, "consentUrl": value, "developmentOnly": true})
}

func (s *Server) consentDetails(w http.ResponseWriter, r *http.Request) {
	req, err := s.requestForConsentToken(r.PathValue("token"))
	if err != nil {
		s.writeConsentError(w, err, actionConsentTokenRejected)
		return
	}
	writeJSON(w, http.StatusOK, consentResponse(req))
}

func (s *Server) approveConsent(w http.ResponseWriter, r *http.Request) {
	s.decideConsent(w, r, "APPROVED")
}
func (s *Server) denyConsent(w http.ResponseWriter, r *http.Request) { s.decideConsent(w, r, "DENIED") }

func (s *Server) decideConsent(w http.ResponseWriter, r *http.Request, decision string) {
	tokenHash := hashToken(r.PathValue("token"))
	action := actionConsentApproved
	if decision == "DENIED" {
		action = actionConsentDenied
	}
	req, err := s.store.VerificationRequestByTokenHash(tokenHash)
	if err != nil {
		s.writeConsentError(w, err, action)
		return
	}
	event := citizenAuditEvent(req, action, "SUCCESS", decision, http.StatusOK)
	updated, err := s.store.DecideConsent(tokenHash, decision, event)
	if err != nil {
		s.writeConsentError(w, err, action)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": updated.ID, "state": updated.State, "decisionAt": updated.DecisionAt})
}

func (s *Server) requestForConsentToken(token string) (store.VerificationRequest, error) {
	if token == "" {
		return store.VerificationRequest{}, store.ErrNotFound
	}
	req, err := s.store.VerificationRequestByTokenHash(hashToken(token))
	if err != nil {
		return req, err
	}
	expires, err := time.Parse(time.RFC3339Nano, req.ExpiresAt)
	if err != nil {
		return req, err
	}
	if !time.Now().UTC().Before(expires) {
		event := citizenAuditEvent(req, actionRequestExpired, "SUCCESS", "EXPIRED", http.StatusGone)
		_, expireErr := s.store.ExpireVerificationRequest(req.ID, event)
		if expireErr != nil {
			return req, expireErr
		}
		return req, store.ErrExpired
	}
	if req.State != "PENDING" {
		return req, store.ErrConflict
	}
	return req, nil
}

func (s *Server) authorizedVerificationRequest(w http.ResponseWriter, r *http.Request, id string) (store.VerificationRequest, bool) {
	principal, _ := rbac.PrincipalFromContext(r.Context())
	req, err := s.store.VerificationRequestByID(id)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "verification request not found")
		return req, false
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not load verification request")
		return req, false
	}
	if req.DepartmentID != principal.Profile.Department.ID || (principal.Profile.Role == string(rbac.RoleOfficial) && req.RequesterUserID != principal.Profile.SupabaseUserID) {
		writeErr(w, http.StatusForbidden, "forbidden")
		return req, false
	}
	s.expireIfNeeded(&req)
	return req, true
}

func (s *Server) expireIfNeeded(req *store.VerificationRequest) {
	if req.State != "PENDING" {
		return
	}
	expires, err := time.Parse(time.RFC3339Nano, req.ExpiresAt)
	if err != nil || time.Now().UTC().Before(expires) {
		return
	}
	event := citizenAuditEvent(*req, actionRequestExpired, "SUCCESS", "EXPIRED", http.StatusGone)
	if changed, err := s.store.ExpireVerificationRequest(req.ID, event); err == nil && changed {
		req.State = "EXPIRED"
		req.Version++
	}
}

func requestAuditEvent(principal *rbac.Principal, req store.VerificationRequest, action, outcome, result string, status int) store.AuditEvent {
	docID, docHash := req.DocumentID, req.ReferenceHash
	return store.AuditEvent{Actor: store.AuditActor{ID: principal.Profile.SupabaseUserID, Email: principal.Profile.Email, Name: principal.Profile.DisplayName, Role: principal.Profile.Role}, Department: &store.AuditDepartment{ID: req.DepartmentID, Name: req.DepartmentName}, Action: action, Outcome: outcome, Result: result, RequestID: req.ID, HTTPStatus: status, Credential: store.AuditCredential{DocID: &docID, DocHash: &docHash}, Details: map[string]any{"verificationRequestId": req.ID}}
}

func citizenAuditEvent(req store.VerificationRequest, action, outcome, result string, status int) store.AuditEvent {
	docID, docHash := req.DocumentID, req.ReferenceHash
	return store.AuditEvent{Actor: store.AuditActor{ID: "citizen:" + req.CitizenAccountID, Role: "CITIZEN_CONSENT"}, Department: &store.AuditDepartment{ID: req.DepartmentID, Name: req.DepartmentName}, Action: action, Outcome: outcome, Result: result, RequestID: req.ID, HTTPStatus: status, Credential: store.AuditCredential{DocID: &docID, DocHash: &docHash}, Details: map[string]any{"verificationRequestId": req.ID, "channel": "EMAIL_LINK"}}
}

func consentResponse(req store.VerificationRequest) map[string]any {
	return map[string]any{"requestId": req.ID, "state": req.State, "requester": map[string]any{"name": req.RequesterName, "role": req.RequesterRole}, "department": map[string]any{"id": req.DepartmentID, "name": req.DepartmentName}, "documentType": req.DocumentType, "purpose": req.Purpose, "expiresAt": req.ExpiresAt}
}

func (s *Server) writeConsentError(w http.ResponseWriter, err error, action string) {
	status, message, outcome := http.StatusInternalServerError, "could not process consent", "FAILURE"
	switch {
	case errors.Is(err, store.ErrNotFound):
		status, message = http.StatusNotFound, "invalid consent link"
	case errors.Is(err, store.ErrExpired):
		status, message = http.StatusGone, "consent link expired"
	case errors.Is(err, store.ErrConflict):
		status, message, outcome = http.StatusConflict, "consent link was already used or request was decided", "DENIED"
	}
	event := store.AuditEvent{Actor: store.AuditActor{ID: "anonymous-consent-link", Role: "CITIZEN_CONSENT"}, Action: action, Outcome: outcome, Result: "REJECTED", HTTPStatus: status, Details: map[string]any{"reason": message}}
	_, _ = s.store.AppendAuditEvent(event)
	writeErr(w, status, message)
}

func (s *Server) auditDeniedOnly(action string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := rbac.PrincipalFromContext(r.Context())
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		recorder := &auditResponseWriter{ResponseWriter: w}
		next.ServeHTTP(recorder, r)
		if recorder.status != http.StatusForbidden {
			return
		}
		event := store.AuditEvent{Actor: store.AuditActor{ID: principal.Profile.SupabaseUserID, Email: principal.Profile.Email, Name: principal.Profile.DisplayName, Role: principal.Profile.Role}, Action: action, Outcome: "DENIED", Result: "DENIED", HTTPStatus: http.StatusForbidden, Details: map[string]any{"reason": "request authorization denied"}}
		if principal.Profile.Department != nil {
			event.Department = &store.AuditDepartment{ID: principal.Profile.Department.ID, Name: principal.Profile.Department.DisplayName}
		}
		_, _ = s.store.AppendAuditEvent(event)
	})
}

func newConsentToken() (string, string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	return token, hashToken(token), nil
}
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
func maskEmail(email string) string {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 {
		return "***"
	}
	local := parts[0]
	if len(local) > 1 {
		local = local[:1]
	}
	return local + "***@" + parts[1]
}
func validRequestState(value string) bool {
	switch value {
	case "PENDING", "APPROVED", "DENIED", "EXPIRED", "COMPLETED":
		return true
	}
	return false
}
