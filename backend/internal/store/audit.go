package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const zeroAuditHash = "0000000000000000000000000000000000000000000000000000000000000000"

type AuditActor struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

type AuditDepartment struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type AuditCredential struct {
	DocID           *string `json:"docId"`
	DocHash         *string `json:"docHash"`
	Citizen         *string `json:"citizen"`
	TransactionHash *string `json:"transactionHash"`
	ReferenceHash   *string `json:"referenceHash"`
}

type AuditEvent struct {
	ID           int64            `json:"id"`
	EventID      string           `json:"eventId"`
	CreatedAt    string           `json:"createdAt"`
	Actor        AuditActor       `json:"actor"`
	Department   *AuditDepartment `json:"department"`
	Action       string           `json:"action"`
	Outcome      string           `json:"outcome"`
	Result       string           `json:"result"`
	Credential   AuditCredential  `json:"credential"`
	RequestID    string           `json:"requestId"`
	HTTPStatus   int              `json:"httpStatus"`
	Error        *string          `json:"error"`
	Details      map[string]any   `json:"details"`
	PreviousHash string           `json:"previousHash"`
	EntryHash    string           `json:"entryHash"`
}

type AuditQuery struct {
	Limit        int
	Before       int64
	Action       string
	Outcome      string
	DepartmentID string
	DocumentID   string
	ActorUserID  string
}

type AuditPage struct {
	Events     []AuditEvent
	HasMore    bool
	NextBefore int64
}

type AuditIntegrity struct {
	Valid               bool   `json:"valid"`
	EventCount          int64  `json:"eventCount"`
	FirstInvalidEventID *int64 `json:"firstInvalidEventId"`
}

type auditConnection interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

// AppendAuditEvent serializes each insert so previous_hash always names the
// immediately preceding row. The event fields are snapshotted and deliberately
// have no update/delete API.
func (s *Store) AppendAuditEvent(event AuditEvent) (AuditEvent, error) {
	conn, err := s.db.Conn(context.Background())
	if err != nil {
		return AuditEvent{}, err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(context.Background(), `BEGIN IMMEDIATE`); err != nil {
		return AuditEvent{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(context.Background(), `ROLLBACK`)
		}
	}()

	event, err = appendAuditEvent(context.Background(), conn, event)
	if err != nil {
		return AuditEvent{}, err
	}
	if _, err := conn.ExecContext(context.Background(), `COMMIT`); err != nil {
		return AuditEvent{}, err
	}
	committed = true
	return event, nil
}

func appendAuditEvent(ctx context.Context, conn auditConnection, event AuditEvent) (AuditEvent, error) {
	if event.EventID == "" {
		event.EventID = randomID()
	}
	if event.RequestID == "" {
		event.RequestID = randomID()
	}
	if event.CreatedAt == "" {
		event.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if event.Details == nil {
		event.Details = map[string]any{}
	}
	details, err := json.Marshal(event.Details)
	if err != nil {
		return AuditEvent{}, fmt.Errorf("encode audit details: %w", err)
	}
	event.PreviousHash = zeroAuditHash
	if err := conn.QueryRowContext(ctx, `SELECT entry_hash FROM audit_logs ORDER BY id DESC LIMIT 1`).Scan(&event.PreviousHash); err != nil && err != sql.ErrNoRows {
		return AuditEvent{}, err
	}
	event.EntryHash = auditHash(event, string(details))
	var departmentID, departmentName any
	if event.Department != nil {
		departmentID, departmentName = event.Department.ID, event.Department.Name
	}
	result, err := conn.ExecContext(ctx, `
		INSERT INTO audit_logs (
		  event_id, created_at, actor_user_id, actor_email, actor_name, actor_role,
		  department_id, department_name, action, outcome, result, document_id,
		  document_hash, citizen, transaction_hash, reference_hash, request_id,
		  http_status, error_message, details_json, previous_hash, entry_hash
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		event.EventID, event.CreatedAt, event.Actor.ID, event.Actor.Email, event.Actor.Name, event.Actor.Role,
		departmentID, departmentName, event.Action, event.Outcome, event.Result,
		event.Credential.DocID, event.Credential.DocHash, event.Credential.Citizen,
		event.Credential.TransactionHash, event.Credential.ReferenceHash, event.RequestID,
		event.HTTPStatus, event.Error, string(details), event.PreviousHash, event.EntryHash,
	)
	if err != nil {
		return AuditEvent{}, err
	}
	event.ID, err = result.LastInsertId()
	if err != nil {
		return AuditEvent{}, err
	}
	return event, nil
}

func (s *Store) AuditEvents(q AuditQuery) (AuditPage, error) {
	limit := q.Limit
	if limit == 0 {
		limit = 50
	}
	where := []string{"1=1"}
	args := []any{}
	add := func(column, value string) {
		if value != "" {
			where = append(where, column+" = ?")
			args = append(args, value)
		}
	}
	if q.Before > 0 {
		where = append(where, "id < ?")
		args = append(args, q.Before)
	}
	add("action", q.Action)
	add("outcome", q.Outcome)
	add("department_id", q.DepartmentID)
	add("document_id", q.DocumentID)
	add("actor_user_id", q.ActorUserID)
	args = append(args, limit+1)
	rows, err := s.db.Query(`SELECT `+auditSelectCols+` FROM audit_logs WHERE `+
		strings.Join(where, " AND ")+` ORDER BY id DESC LIMIT ?`, args...)
	if err != nil {
		return AuditPage{}, err
	}
	defer rows.Close()
	events := []AuditEvent{}
	for rows.Next() {
		event, err := scanAudit(rows)
		if err != nil {
			return AuditPage{}, err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return AuditPage{}, err
	}
	page := AuditPage{Events: events}
	if len(events) > limit {
		page.HasMore = true
		page.Events = events[:limit]
	}
	if len(page.Events) > 0 {
		page.NextBefore = page.Events[len(page.Events)-1].ID
	}
	return page, nil
}

func (s *Store) VerifyAuditChain() (AuditIntegrity, error) {
	rows, err := s.db.Query(`SELECT ` + auditSelectCols + ` FROM audit_logs ORDER BY id`)
	if err != nil {
		return AuditIntegrity{}, err
	}
	defer rows.Close()
	integrity := AuditIntegrity{Valid: true}
	previous := zeroAuditHash
	for rows.Next() {
		event, detailsJSON, err := scanAuditRaw(rows)
		if err != nil {
			return AuditIntegrity{}, err
		}
		integrity.EventCount++
		if event.PreviousHash != previous || event.EntryHash != auditHash(event, detailsJSON) {
			integrity.Valid = false
			id := event.ID
			integrity.FirstInvalidEventID = &id
			return integrity, nil
		}
		previous = event.EntryHash
	}
	return integrity, rows.Err()
}

const auditSelectCols = `id, event_id, created_at, actor_user_id, actor_email, actor_name, actor_role,
 department_id, department_name, action, outcome, result, document_id, document_hash, citizen,
 transaction_hash, reference_hash, request_id, http_status, error_message, details_json,
 previous_hash, entry_hash`

func scanAudit(scanner interface{ Scan(...any) error }) (AuditEvent, error) {
	event, _, err := scanAuditRaw(scanner)
	return event, err
}

func scanAuditRaw(scanner interface{ Scan(...any) error }) (AuditEvent, string, error) {
	var event AuditEvent
	var departmentID, departmentName sql.NullString
	var docID, docHash, citizen, txHash, referenceHash, errorMessage sql.NullString
	var detailsJSON string
	err := scanner.Scan(
		&event.ID, &event.EventID, &event.CreatedAt, &event.Actor.ID, &event.Actor.Email,
		&event.Actor.Name, &event.Actor.Role, &departmentID, &departmentName, &event.Action,
		&event.Outcome, &event.Result, &docID, &docHash, &citizen, &txHash, &referenceHash,
		&event.RequestID, &event.HTTPStatus, &errorMessage, &detailsJSON,
		&event.PreviousHash, &event.EntryHash,
	)
	if err != nil {
		return AuditEvent{}, "", err
	}
	if departmentID.Valid {
		event.Department = &AuditDepartment{ID: departmentID.String, Name: departmentName.String}
	}
	event.Credential = AuditCredential{
		DocID: stringPtr(docID), DocHash: stringPtr(docHash), Citizen: stringPtr(citizen),
		TransactionHash: stringPtr(txHash), ReferenceHash: stringPtr(referenceHash),
	}
	event.Error = stringPtr(errorMessage)
	if err := json.Unmarshal([]byte(detailsJSON), &event.Details); err != nil {
		return AuditEvent{}, "", fmt.Errorf("decode audit details: %w", err)
	}
	return event, detailsJSON, nil
}

func stringPtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}

func auditHash(event AuditEvent, detailsJSON string) string {
	departmentID, departmentName := "", ""
	if event.Department != nil {
		departmentID, departmentName = event.Department.ID, event.Department.Name
	}
	value := struct {
		Version                                                           int
		EventID, CreatedAt, ActorID, ActorEmail, ActorName, ActorRole     string
		DepartmentID, DepartmentName, Action, Outcome, Result             string
		DocumentID, DocumentHash, Citizen, TransactionHash, ReferenceHash string
		RequestID                                                         string
		HTTPStatus                                                        int
		Error, DetailsJSON, PreviousHash                                  string
	}{
		1, event.EventID, event.CreatedAt, event.Actor.ID, event.Actor.Email, event.Actor.Name,
		event.Actor.Role, departmentID, departmentName, event.Action, event.Outcome, event.Result,
		pointerValue(event.Credential.DocID), pointerValue(event.Credential.DocHash),
		pointerValue(event.Credential.Citizen), pointerValue(event.Credential.TransactionHash),
		pointerValue(event.Credential.ReferenceHash), event.RequestID, event.HTTPStatus,
		pointerValue(event.Error), detailsJSON, event.PreviousHash,
	}
	raw, _ := json.Marshal(value)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func pointerValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func randomID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(value[:])
}
