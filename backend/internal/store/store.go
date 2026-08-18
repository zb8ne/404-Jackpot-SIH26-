// Package store is the off-chain half of the system: the PDF bytes and the
// citizen -> document index that the chain deliberately does not hold.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	_ "modernc.org/sqlite"
)

type Document struct {
	DocHash   string `json:"docHash"`
	DocID     string `json:"docId"`
	DocType   string `json:"docType"`
	Citizen   string `json:"citizen"`
	Issuer    string `json:"issuer"`
	Filename  string `json:"filename"`
	TxHash    string `json:"txHash"`
	IssuedAt  string `json:"issuedAt"`
	SizeBytes int    `json:"sizeBytes"`
}

type Store struct{ db *sql.DB }

type Department struct {
	ID          string `json:"id"`
	DisplayName string `json:"name"`
	DocType     uint8  `json:"docType"`
	Active      bool   `json:"active"`
}

type UserProfile struct {
	SupabaseUserID string      `json:"id"`
	Email          string      `json:"email"`
	DisplayName    string      `json:"name"`
	Role           string      `json:"role"`
	Department     *Department `json:"department"`
	Active         bool        `json:"active"`
}

var ErrNotFound = errors.New("not found")

const schema = `
CREATE TABLE IF NOT EXISTS documents (
  doc_hash   TEXT PRIMARY KEY,
  doc_id     TEXT NOT NULL,
  doc_type   TEXT NOT NULL,
  citizen    TEXT NOT NULL,
  issuer     TEXT NOT NULL,
  filename   TEXT NOT NULL,
  tx_hash    TEXT NOT NULL,
  issued_at  TEXT NOT NULL DEFAULT (datetime('now')),
  pdf        BLOB NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_documents_citizen ON documents(citizen);

CREATE TABLE IF NOT EXISTS departments (
  id           TEXT PRIMARY KEY,
  display_name TEXT NOT NULL,
  doc_type     INTEGER NOT NULL UNIQUE,
  active       INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0, 1))
);

CREATE TABLE IF NOT EXISTS user_profiles (
  supabase_user_id TEXT PRIMARY KEY,
  email            TEXT NOT NULL,
  display_name     TEXT NOT NULL DEFAULT '',
  role             TEXT NOT NULL CHECK (role IN ('CONTROLLER', 'ADMIN', 'OFFICIAL')),
  department_id    TEXT REFERENCES departments(id),
  active           INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0, 1)),
  CHECK (
    (role = 'CONTROLLER' AND department_id IS NULL)
    OR
    (role IN ('ADMIN', 'OFFICIAL') AND department_id IS NOT NULL)
  )
);
CREATE INDEX IF NOT EXISTS idx_user_profiles_department ON user_profiles(department_id);

CREATE TABLE IF NOT EXISTS audit_logs (
  id                INTEGER PRIMARY KEY AUTOINCREMENT,
  event_id          TEXT NOT NULL UNIQUE,
  created_at        TEXT NOT NULL,
  actor_user_id     TEXT NOT NULL,
  actor_email       TEXT NOT NULL DEFAULT '',
  actor_name        TEXT NOT NULL DEFAULT '',
  actor_role        TEXT NOT NULL,
  department_id     TEXT,
  department_name   TEXT,
  action            TEXT NOT NULL,
  outcome           TEXT NOT NULL CHECK (outcome IN ('SUCCESS', 'FAILURE', 'DENIED', 'PARTIAL_FAILURE')),
  result            TEXT NOT NULL DEFAULT '',
  document_id       TEXT,
  document_hash     TEXT,
  citizen           TEXT,
  transaction_hash  TEXT,
  reference_hash    TEXT,
  request_id        TEXT NOT NULL,
  http_status       INTEGER NOT NULL,
  error_message     TEXT,
  details_json      TEXT NOT NULL DEFAULT '{}',
  previous_hash     TEXT NOT NULL,
  entry_hash        TEXT NOT NULL UNIQUE
);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created ON audit_logs(id DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_department_created ON audit_logs(department_id, id DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_action_created ON audit_logs(action, id DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_outcome_created ON audit_logs(outcome, id DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_document_id ON audit_logs(document_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_document_hash ON audit_logs(document_hash);
CREATE INDEX IF NOT EXISTS idx_audit_logs_actor_created ON audit_logs(actor_user_id, id DESC);
`

var defaultDepartments = []Department{
	{ID: "birth", DisplayName: "Birth Registration Dept", DocType: 1, Active: true},
	{ID: "transport", DisplayName: "Transport Dept", DocType: 2, Active: true},
	{ID: "education", DisplayName: "Education Dept", DocType: 3, Active: true},
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	// PRAGMA foreign_keys is connection-local. This small SQLite-backed service
	// deliberately uses one connection so every operation observes the same
	// constraint setting.
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	for _, d := range defaultDepartments {
		if _, err := db.Exec(
			`INSERT OR IGNORE INTO departments (id, display_name, doc_type, active) VALUES (?, ?, ?, ?)`,
			d.ID, d.DisplayName, d.DocType, d.Active,
		); err != nil {
			db.Close()
			return nil, fmt.Errorf("initialize department %s: %w", d.ID, err)
		}
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Save(d Document, pdf []byte) error {
	_, err := s.db.Exec(
		`INSERT INTO documents (doc_hash, doc_id, doc_type, citizen, issuer, filename, tx_hash, pdf)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		d.DocHash, d.DocID, d.DocType, d.Citizen, d.Issuer, d.Filename, d.TxHash, pdf,
	)
	return err
}

const selectCols = `doc_hash, doc_id, doc_type, citizen, issuer, filename, tx_hash, issued_at, length(pdf)`

func scan(rows interface{ Scan(...any) error }) (Document, error) {
	var d Document
	err := rows.Scan(&d.DocHash, &d.DocID, &d.DocType, &d.Citizen, &d.Issuer,
		&d.Filename, &d.TxHash, &d.IssuedAt, &d.SizeBytes)
	return d, err
}

// ByCitizen returns every document ever handed to a citizen, newest first.
// Current status is not stored here — the caller asks the chain for that.
func (s *Store) ByCitizen(citizen string) ([]Document, error) {
	return s.byCitizen(citizen, "", false)
}

func (s *Store) ByCitizenAndDocType(citizen, docType string) ([]Document, error) {
	return s.byCitizen(citizen, docType, true)
}

func (s *Store) byCitizen(citizen, docType string, scoped bool) ([]Document, error) {
	query := `SELECT ` + selectCols + ` FROM documents WHERE citizen = ?`
	args := []any{citizen}
	if scoped {
		query += ` AND doc_type = ?`
		args = append(args, docType)
	}
	query += ` ORDER BY issued_at DESC, rowid DESC`
	rows, err := s.db.Query(
		query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	docs := []Document{}
	for rows.Next() {
		d, err := scan(rows)
		if err != nil {
			return nil, err
		}
		docs = append(docs, d)
	}
	return docs, rows.Err()
}

// ByHash looks up a single document. Returns ok=false when the hash is unknown,
// which for a hash taken off a real PDF means the file was tampered with.
func (s *Store) ByHash(docHash string) (Document, bool, error) {
	row := s.db.QueryRow(`SELECT `+selectCols+` FROM documents WHERE doc_hash = ?`, docHash)
	d, err := scan(row)
	if err == sql.ErrNoRows {
		return Document{}, false, nil
	}
	if err != nil {
		return Document{}, false, err
	}
	return d, true, nil
}

func (s *Store) PDF(docHash string) ([]byte, bool, error) {
	var pdf []byte
	err := s.db.QueryRow(`SELECT pdf FROM documents WHERE doc_hash = ?`, docHash).Scan(&pdf)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return pdf, true, nil
}

func (s *Store) Citizens() ([]string, error) {
	return s.citizens("", false)
}

func (s *Store) CitizensByDocType(docType string) ([]string, error) {
	return s.citizens(docType, true)
}

func (s *Store) citizens(docType string, scoped bool) ([]string, error) {
	query := `SELECT DISTINCT citizen FROM documents`
	args := []any{}
	if scoped {
		query += ` WHERE doc_type = ?`
		args = append(args, docType)
	}
	query += ` ORDER BY citizen`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	names := []string{}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		names = append(names, n)
	}
	return names, rows.Err()
}

func (s *Store) Departments() ([]Department, error) {
	rows, err := s.db.Query(`SELECT id, display_name, doc_type, active FROM departments ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	departments := []Department{}
	for rows.Next() {
		var d Department
		if err := rows.Scan(&d.ID, &d.DisplayName, &d.DocType, &d.Active); err != nil {
			return nil, err
		}
		departments = append(departments, d)
	}
	return departments, rows.Err()
}

func (s *Store) DepartmentByID(id string) (Department, error) {
	var d Department
	err := s.db.QueryRow(
		`SELECT id, display_name, doc_type, active FROM departments WHERE id = ?`, id,
	).Scan(&d.ID, &d.DisplayName, &d.DocType, &d.Active)
	if err == sql.ErrNoRows {
		return Department{}, ErrNotFound
	}
	return d, err
}

func (s *Store) UserProfileByID(id string) (UserProfile, error) {
	return userProfileByID(context.Background(), s.db, id)
}

type profileQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func userProfileByID(ctx context.Context, queryer profileQueryer, id string) (UserProfile, error) {
	var p UserProfile
	var departmentID, departmentName sql.NullString
	var departmentDocType sql.NullInt64
	var departmentActive sql.NullBool
	err := queryer.QueryRowContext(ctx, `
		SELECT p.supabase_user_id, p.email, p.display_name, p.role, p.active,
		       d.id, d.display_name, d.doc_type, d.active
		FROM user_profiles p
		LEFT JOIN departments d ON d.id = p.department_id
		WHERE p.supabase_user_id = ?`, id,
	).Scan(
		&p.SupabaseUserID, &p.Email, &p.DisplayName, &p.Role, &p.Active,
		&departmentID, &departmentName, &departmentDocType, &departmentActive,
	)
	if err == sql.ErrNoRows {
		return UserProfile{}, ErrNotFound
	}
	if err != nil {
		return UserProfile{}, err
	}
	if departmentID.Valid {
		p.Department = &Department{
			ID:          departmentID.String,
			DisplayName: departmentName.String,
			DocType:     uint8(departmentDocType.Int64),
			Active:      departmentActive.Bool,
		}
	}
	return p, nil
}

func upsertUserProfile(ctx context.Context, conn auditConnection, p UserProfile, departmentID string) error {
	var department any
	if departmentID != "" {
		department = departmentID
	}
	_, err := conn.ExecContext(ctx, `
		INSERT INTO user_profiles
		  (supabase_user_id, email, display_name, role, department_id, active)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(supabase_user_id) DO UPDATE SET
		  email = excluded.email,
		  display_name = excluded.display_name,
		  role = excluded.role,
		  department_id = excluded.department_id,
		  active = excluded.active`,
		p.SupabaseUserID, p.Email, p.DisplayName, p.Role, department, p.Active,
	)
	return err
}

// UpsertUserProfileAudited is the supported administrative write path. The
// audit event snapshots both sides of the authorization change so a role,
// department, or active-state edit cannot be silent at the application layer.
func (s *Store) UpsertUserProfileAudited(p UserProfile, departmentID string, actor AuditActor) error {
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

	before, beforeErr := userProfileByID(ctx, conn, p.SupabaseUserID)
	if beforeErr != nil && beforeErr != ErrNotFound {
		return beforeErr
	}
	if err := upsertUserProfile(ctx, conn, p, departmentID); err != nil {
		return err
	}
	after := p
	if departmentID != "" {
		after.Department = &Department{ID: departmentID}
	}
	action := "USER_PROFILE_UPDATE"
	if beforeErr == ErrNotFound {
		action = "USER_PROFILE_CREATE"
	}
	details := map[string]any{
		"targetUserId": p.SupabaseUserID,
		"before":       profileAuditValue(before, beforeErr == nil),
		"after":        profileAuditValue(after, true),
		"departmentId": departmentID,
	}
	if _, err := appendAuditEvent(ctx, conn, AuditEvent{
		Actor: actor, Action: action, Outcome: "SUCCESS", Result: p.Role,
		RequestID: randomID(), HTTPStatus: 0, Details: details,
	}); err != nil {
		return fmt.Errorf("authorization audit failed; profile change rolled back: %w", err)
	}
	if _, err := conn.ExecContext(ctx, `COMMIT`); err != nil {
		return err
	}
	committed = true
	return nil
}

func profileAuditValue(p UserProfile, present bool) any {
	if !present {
		return nil
	}
	departmentID := ""
	if p.Department != nil {
		departmentID = p.Department.ID
	}
	return map[string]any{
		"email": p.Email, "name": p.DisplayName, "role": p.Role,
		"departmentId": departmentID, "active": p.Active,
	}
}
