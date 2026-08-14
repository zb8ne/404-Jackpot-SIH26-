// Package store is the off-chain half of the system: the PDF bytes and the
// citizen -> document index that the chain deliberately does not hold.
package store

import (
	"database/sql"
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
`

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
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
	rows, err := s.db.Query(
		`SELECT `+selectCols+` FROM documents WHERE citizen = ? ORDER BY issued_at DESC, rowid DESC`,
		citizen)
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
	rows, err := s.db.Query(`SELECT DISTINCT citizen FROM documents ORDER BY citizen`)
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
