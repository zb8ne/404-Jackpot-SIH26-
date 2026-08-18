package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/mail"
	"strings"
	"time"
)

var (
	ErrConflict = errors.New("conflicting verification request state")
	ErrExpired  = errors.New("verification request expired")
)

type CitizenAccount struct {
	ID             string  `json:"id"`
	SupabaseUserID *string `json:"supabaseUserId"`
	DisplayName    string  `json:"displayName"`
	Email          string  `json:"email"`
	Phone          *string `json:"phone"`
	Active         bool    `json:"active"`
	CreatedAt      string  `json:"createdAt"`
	UpdatedAt      string  `json:"updatedAt"`
}

func (s *Store) UpsertCitizenAccount(account CitizenAccount) error {
	if strings.TrimSpace(account.ID) == "" || strings.TrimSpace(account.DisplayName) == "" || strings.TrimSpace(account.Email) == "" {
		return errors.New("citizen id, display name, and email are required")
	}
	parsed, err := mail.ParseAddress(strings.TrimSpace(account.Email))
	if err != nil || parsed.Address != strings.TrimSpace(account.Email) {
		return errors.New("citizen email is invalid")
	}
	_, err = s.db.Exec(`
		INSERT INTO citizen_accounts (id, supabase_user_id, display_name, email, phone, active)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET supabase_user_id=excluded.supabase_user_id,
		 display_name=excluded.display_name, email=excluded.email, phone=excluded.phone,
		 active=excluded.active, updated_at=datetime('now')`,
		account.ID, account.SupabaseUserID, account.DisplayName, account.Email, account.Phone, account.Active)
	return err
}

func (s *Store) CitizenAccountByID(id string) (CitizenAccount, error) {
	return scanCitizen(s.db.QueryRow(`SELECT id, supabase_user_id, display_name, email, phone, active, created_at, updated_at FROM citizen_accounts WHERE id=?`, id))
}

func (s *Store) CitizenAccounts() ([]CitizenAccount, error) {
	rows, err := s.db.Query(`SELECT id, supabase_user_id, display_name, email, phone, active, created_at, updated_at FROM citizen_accounts WHERE active=1 ORDER BY display_name, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	accounts := []CitizenAccount{}
	for rows.Next() {
		a, err := scanCitizen(rows)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, a)
	}
	return accounts, rows.Err()
}

func scanCitizen(scanner interface{ Scan(...any) error }) (CitizenAccount, error) {
	var a CitizenAccount
	var supabaseID, phone sql.NullString
	err := scanner.Scan(&a.ID, &supabaseID, &a.DisplayName, &a.Email, &phone, &a.Active, &a.CreatedAt, &a.UpdatedAt)
	if err == sql.ErrNoRows {
		return CitizenAccount{}, ErrNotFound
	}
	if err != nil {
		return CitizenAccount{}, err
	}
	if supabaseID.Valid {
		a.SupabaseUserID = &supabaseID.String
	}
	if phone.Valid {
		a.Phone = &phone.String
	}
	return a, nil
}

type VerificationRequest struct {
	ID                      string         `json:"id"`
	DocumentID              string         `json:"documentId"`
	ReferenceHash           string         `json:"-"`
	CitizenAccountID        string         `json:"-"`
	RequesterUserID         string         `json:"requesterUserId"`
	RequesterEmail          string         `json:"requesterEmail"`
	RequesterName           string         `json:"requesterName"`
	RequesterRole           string         `json:"requesterRole"`
	DepartmentID            string         `json:"departmentId"`
	DepartmentName          string         `json:"departmentName"`
	DocumentType            string         `json:"documentType"`
	State                   string         `json:"state"`
	Purpose                 string         `json:"purpose"`
	CreatedAt               string         `json:"createdAt"`
	ExpiresAt               string         `json:"expiresAt"`
	DecisionAt              *string        `json:"decisionAt"`
	CompletedAt             *string        `json:"completedAt"`
	ApprovalTokenHash       *string        `json:"-"`
	DecisionChannel         *string        `json:"decisionChannel"`
	DecisionReference       *string        `json:"decisionReference"`
	CompletedResult         map[string]any `json:"completedResult"`
	Version                 int            `json:"version"`
	NotificationStatus      string         `json:"notificationStatus"`
	NotificationDestination string         `json:"notificationDestination"`
}

type VerificationRequestQuery struct {
	Limit, Offset                        int
	State, RequesterUserID, DepartmentID string
}

func (s *Store) CreateVerificationRequest(req VerificationRequest, event AuditEvent) error {
	return s.withImmediate(func(ctx context.Context, conn *sql.Conn) error {
		_, err := conn.ExecContext(ctx, `INSERT INTO verification_requests (
		 id, document_id, reference_hash, citizen_account_id, requester_user_id,
		 requester_email, requester_name, requester_role, department_id, department_name,
		 document_type, state, purpose, created_at, expires_at, approval_token_hash)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'PENDING', ?, ?, ?, ?)`,
			req.ID, req.DocumentID, req.ReferenceHash, req.CitizenAccountID, req.RequesterUserID,
			req.RequesterEmail, req.RequesterName, req.RequesterRole, req.DepartmentID,
			req.DepartmentName, req.DocumentType, req.Purpose, req.CreatedAt, req.ExpiresAt,
			req.ApprovalTokenHash)
		if err != nil {
			return err
		}
		_, err = appendAuditEvent(ctx, conn, event)
		return err
	})
}

func (s *Store) VerificationRequestByID(id string) (VerificationRequest, error) {
	return scanVerificationRequest(s.db.QueryRow(verificationRequestSelect+` WHERE vr.id=?`, id))
}

func (s *Store) VerificationRequestByTokenHash(hash string) (VerificationRequest, error) {
	return scanVerificationRequest(s.db.QueryRow(verificationRequestSelect+` WHERE vr.approval_token_hash=?`, hash))
}

func (s *Store) VerificationRequests(q VerificationRequestQuery) ([]VerificationRequest, error) {
	where := []string{"1=1"}
	args := []any{}
	if q.State != "" {
		where = append(where, "vr.state=?")
		args = append(args, q.State)
	}
	if q.RequesterUserID != "" {
		where = append(where, "vr.requester_user_id=?")
		args = append(args, q.RequesterUserID)
	}
	if q.DepartmentID != "" {
		where = append(where, "vr.department_id=?")
		args = append(args, q.DepartmentID)
	}
	limit := q.Limit
	if limit == 0 {
		limit = 50
	}
	args = append(args, limit, q.Offset)
	rows, err := s.db.Query(verificationRequestSelect+` WHERE `+strings.Join(where, " AND ")+` ORDER BY vr.created_at DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	requests := []VerificationRequest{}
	for rows.Next() {
		req, err := scanVerificationRequest(rows)
		if err != nil {
			return nil, err
		}
		requests = append(requests, req)
	}
	return requests, rows.Err()
}

func (s *Store) RecordNotification(requestID, destination, status, errorMessage string, event AuditEvent) error {
	return s.withImmediate(func(ctx context.Context, conn *sql.Conn) error {
		var failure any
		if errorMessage != "" {
			failure = errorMessage
		}
		_, err := conn.ExecContext(ctx, `INSERT INTO notification_attempts (verification_request_id, channel, destination_redacted, status, error_message, created_at) VALUES (?, 'EMAIL', ?, ?, ?, ?)`, requestID, destination, status, failure, time.Now().UTC().Format(time.RFC3339Nano))
		if err != nil {
			return err
		}
		_, err = appendAuditEvent(ctx, conn, event)
		return err
	})
}

func (s *Store) DecideConsent(tokenHash, decision string, event AuditEvent) (VerificationRequest, error) {
	var result VerificationRequest
	var decisionErr error
	err := s.withImmediate(func(ctx context.Context, conn *sql.Conn) error {
		req, err := scanVerificationRequest(conn.QueryRowContext(ctx, verificationRequestSelect+` WHERE vr.approval_token_hash=?`, tokenHash))
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		if req.State == decision {
			result = req
			return nil
		}
		if req.State == "EXPIRED" {
			decisionErr = ErrExpired
			return nil
		}
		if req.State != "PENDING" {
			return ErrConflict
		}
		expires, err := time.Parse(time.RFC3339Nano, req.ExpiresAt)
		if err != nil {
			return err
		}
		if !now.Before(expires) {
			_, err = conn.ExecContext(ctx, `UPDATE verification_requests SET state='EXPIRED', version=version+1 WHERE id=? AND state='PENDING'`, req.ID)
			if err != nil {
				return err
			}
			event.Action, event.Result = "VERIFICATION_REQUEST_EXPIRED", "EXPIRED"
			_, err = appendAuditEvent(ctx, conn, event)
			if err != nil {
				return err
			}
			decisionErr = ErrExpired
			return nil
		}
		decisionAt := now.Format(time.RFC3339Nano)
		updated, err := conn.ExecContext(ctx, `UPDATE verification_requests SET state=?, decision_at=?, decision_channel='EMAIL_LINK', decision_reference='one-time-token', version=version+1 WHERE id=? AND state='PENDING' AND approval_token_hash=?`, decision, decisionAt, req.ID, tokenHash)
		if err != nil {
			return err
		}
		count, _ := updated.RowsAffected()
		if count != 1 {
			return ErrConflict
		}
		_, err = appendAuditEvent(ctx, conn, event)
		if err != nil {
			return err
		}
		result, err = scanVerificationRequest(conn.QueryRowContext(ctx, verificationRequestSelect+` WHERE vr.id=?`, req.ID))
		return err
	})
	if err == nil && decisionErr != nil {
		return VerificationRequest{}, decisionErr
	}
	return result, err
}

func (s *Store) ExpireVerificationRequest(id string, event AuditEvent) (bool, error) {
	changed := false
	err := s.withImmediate(func(ctx context.Context, conn *sql.Conn) error {
		result, err := conn.ExecContext(ctx, `UPDATE verification_requests SET state='EXPIRED', version=version+1 WHERE id=? AND state='PENDING' AND expires_at<=?`, id, time.Now().UTC().Format(time.RFC3339Nano))
		if err != nil {
			return err
		}
		count, _ := result.RowsAffected()
		changed = count == 1
		if changed {
			_, err = appendAuditEvent(ctx, conn, event)
		}
		return err
	})
	return changed, err
}

func (s *Store) CompleteVerificationRequest(id string, completed map[string]any, event AuditEvent) (VerificationRequest, error) {
	var result VerificationRequest
	err := s.withImmediate(func(ctx context.Context, conn *sql.Conn) error {
		resultJSON, err := encodeJSON(completed)
		if err != nil {
			return err
		}
		now := time.Now().UTC().Format(time.RFC3339Nano)
		updated, err := conn.ExecContext(ctx, `UPDATE verification_requests SET state='COMPLETED', completed_at=?, completed_result_json=?, version=version+1 WHERE id=? AND state='APPROVED' AND expires_at>?`, now, resultJSON, id, now)
		if err != nil {
			return err
		}
		count, _ := updated.RowsAffected()
		if count != 1 {
			return ErrConflict
		}
		_, err = appendAuditEvent(ctx, conn, event)
		if err != nil {
			return err
		}
		result, err = scanVerificationRequest(conn.QueryRowContext(ctx, verificationRequestSelect+` WHERE vr.id=?`, id))
		return err
	})
	return result, err
}

func (s *Store) withImmediate(fn func(context.Context, *sql.Conn) error) error {
	ctx := context.Background()
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, `BEGIN IMMEDIATE`); err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(ctx, `ROLLBACK`)
		}
	}()
	if err := fn(ctx, conn); err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, `COMMIT`); err != nil {
		return err
	}
	committed = true
	return nil
}

const verificationRequestSelect = `SELECT vr.id, vr.document_id, vr.reference_hash, vr.citizen_account_id,
 vr.requester_user_id, vr.requester_email, vr.requester_name, vr.requester_role,
 vr.department_id, vr.department_name, vr.document_type, vr.state, vr.purpose,
 vr.created_at, vr.expires_at, vr.decision_at, vr.completed_at, vr.approval_token_hash,
 vr.decision_channel, vr.decision_reference, vr.completed_result_json, vr.version,
 COALESCE((SELECT status FROM notification_attempts n WHERE n.verification_request_id=vr.id ORDER BY n.id DESC LIMIT 1), 'PENDING'),
 COALESCE((SELECT destination_redacted FROM notification_attempts n WHERE n.verification_request_id=vr.id ORDER BY n.id DESC LIMIT 1), '')
 FROM verification_requests vr`

func scanVerificationRequest(scanner interface{ Scan(...any) error }) (VerificationRequest, error) {
	var req VerificationRequest
	var decisionAt, completedAt, tokenHash, channel, reference, resultJSON sql.NullString
	err := scanner.Scan(&req.ID, &req.DocumentID, &req.ReferenceHash, &req.CitizenAccountID,
		&req.RequesterUserID, &req.RequesterEmail, &req.RequesterName, &req.RequesterRole,
		&req.DepartmentID, &req.DepartmentName, &req.DocumentType, &req.State, &req.Purpose,
		&req.CreatedAt, &req.ExpiresAt, &decisionAt, &completedAt, &tokenHash, &channel,
		&reference, &resultJSON, &req.Version, &req.NotificationStatus, &req.NotificationDestination)
	if err == sql.ErrNoRows {
		return VerificationRequest{}, ErrNotFound
	}
	if err != nil {
		return VerificationRequest{}, err
	}
	req.DecisionAt, req.CompletedAt, req.ApprovalTokenHash = stringPtr(decisionAt), stringPtr(completedAt), stringPtr(tokenHash)
	req.DecisionChannel, req.DecisionReference = stringPtr(channel), stringPtr(reference)
	if resultJSON.Valid {
		if err := decodeJSON(resultJSON.String, &req.CompletedResult); err != nil {
			return VerificationRequest{}, err
		}
	}
	return req, nil
}

func encodeJSON(value any) (string, error)    { raw, err := json.Marshal(value); return string(raw), err }
func decodeJSON(raw string, target any) error { return json.Unmarshal([]byte(raw), target) }
