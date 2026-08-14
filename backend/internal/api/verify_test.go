package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"testing"

	"credreg/backend/internal/chain"
	"credreg/backend/internal/docid"
	"credreg/backend/internal/store"
)

// fakeRegistry stands in for the contract. The verify state machine has changed
// several times; this pins every verdict down without needing a node.
type fakeRegistry struct {
	records map[[32]byte]chain.Record
	current map[string][32]byte
}

func (f *fakeRegistry) Verify(_ context.Context, h [32]byte) (chain.Record, error) {
	rec, ok := f.records[h]
	if !ok {
		return chain.Record{Found: false}, nil
	}
	rec.Found = true
	return rec, nil
}

func (f *fakeRegistry) CurrentHashOf(_ context.Context, id string) ([32]byte, bool, error) {
	h, ok := f.current[id]
	return h, ok, nil
}

func hashOf(b []byte) ([32]byte, string) {
	sum := sha256.Sum256(b)
	return sum, "0x" + hex.EncodeToString(sum[:])
}

// pdf fakes a document body carrying the docId marker, the same way the seeder
// embeds it.
func pdf(body, id string) []byte {
	return []byte(body + "\n" + docid.Marker + id + "\n")
}

const (
	idValid    = "DEG-2021-1174"
	idRevoked  = "DL-GA-2016-33017"
	idBirthV1  = "BC-2019-004471"
	idBirthV2  = "BC-2019-004471-R1"
	idNotExist = "DL-GA-2024-99999"
)

// world builds a registry holding one document of each interesting kind.
func world(t *testing.T) (*Server, map[string][]byte) {
	t.Helper()

	files := map[string][]byte{
		"valid":    pdf("degree certificate", idValid),
		"revoked":  pdf("driving licence", idRevoked),
		"birthV1":  pdf("birth certificate, name misspelled", idBirthV1),
		"birthV2":  pdf("birth certificate, name corrected", idBirthV2),
		"forged":   pdf("driving licence for someone who has none", idNotExist),
		"tampered": pdf("degree certificate, class upgraded", idValid),
		"unmarked": []byte("a pdf with no registry marker at all"),
	}

	reg := &fakeRegistry{
		records: map[[32]byte]chain.Record{},
		current: map[string][32]byte{},
	}

	add := func(file, id string, docType uint8, status uint8, prev [32]byte) [32]byte {
		sum, _ := hashOf(files[file])
		reg.records[sum] = chain.Record{
			DocID: id, DocType: docType, Issuer: "0xdept", Timestamp: 1700000000,
			Status: status, PrevHash: "0x" + hex.EncodeToString(prev[:]),
		}
		reg.current[id] = sum
		return sum
	}

	add("valid", idValid, chain.DocDegreeCertificate, chain.StatusValid, [32]byte{})
	add("revoked", idRevoked, chain.DocDrivingLicence, chain.StatusRevoked, [32]byte{})

	// v1 superseded by v2: both ids now point at v2, exactly as the contract
	// leaves things after supersede().
	v1 := add("birthV1", idBirthV1, chain.DocBirthCertificate, chain.StatusSuperseded, [32]byte{})
	v2 := add("birthV2", idBirthV2, chain.DocBirthCertificate, chain.StatusValid, v1)
	reg.current[idBirthV1] = v2

	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })

	return &Server{reader: reg, store: st}, files
}

func resolveFile(t *testing.T, s *Server, raw []byte) map[string]any {
	t.Helper()

	_, computed := hashOf(raw)
	id, _ := docid.Extract(raw)

	resp := map[string]any{"expectedHash": zeroHash, "docId": id, "computedHash": computed}
	if err := s.resolve(context.Background(), id, computed, resp); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	return resp
}

func TestVerifyVerdicts(t *testing.T) {
	s, files := world(t)

	cases := []struct {
		name string
		file string
		want string
	}{
		{"genuine and current", "valid", "VALID"},
		{"genuine but revoked", "revoked", "REVOKED"},
		{"the original of a corrected document", "birthV1", "SUPERSEDED"},
		{"the correction itself", "birthV2", "VALID"},
		{"known id, altered bytes", "tampered", "TAMPERED"},
		{"well-formed but never issued", "forged", "NOT_ISSUED"},
		{"no marker and an unknown hash", "unmarked", "NOT_ISSUED"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveFile(t, s, files[tc.file])
			if got["status"] != tc.want {
				t.Errorf("status = %v, want %v (message: %v)", got["status"], tc.want, got["message"])
			}
			// Both hashes are always present, so the UI can show them side by side.
			if got["expectedHash"] == nil || got["computedHash"] == nil {
				t.Errorf("expected both hashes in the response, got %v", got)
			}
		})
	}
}

// The regression this ordering exists to prevent: after a supersede, the
// untouched original must not read as TAMPERED.
func TestSupersededOriginalIsNotTampered(t *testing.T) {
	s, files := world(t)

	got := resolveFile(t, s, files["birthV1"])
	if got["status"] != "SUPERSEDED" {
		t.Fatalf("status = %v, want SUPERSEDED", got["status"])
	}

	// Its own hash is what we expected, not the replacement's.
	_, v1Hash := hashOf(files["birthV1"])
	if got["expectedHash"] != v1Hash {
		t.Errorf("expectedHash = %v, want the original's own hash %v", got["expectedHash"], v1Hash)
	}
	if got["computedHash"] != v1Hash {
		t.Errorf("computedHash = %v, want %v", got["computedHash"], v1Hash)
	}

	// And it points at the version that replaced it.
	by, ok := got["supersededBy"].(map[string]any)
	if !ok {
		t.Fatalf("supersededBy missing, got %v", got)
	}
	_, v2Hash := hashOf(files["birthV2"])
	if by["docId"] != idBirthV2 || by["hash"] != v2Hash {
		t.Errorf("supersededBy = %v, want docId %s and hash %s", by, idBirthV2, v2Hash)
	}
}

func TestTamperedShowsBothHashes(t *testing.T) {
	s, files := world(t)

	got := resolveFile(t, s, files["tampered"])
	if got["status"] != "TAMPERED" {
		t.Fatalf("status = %v, want TAMPERED", got["status"])
	}

	_, tamperedHash := hashOf(files["tampered"])
	_, genuineHash := hashOf(files["valid"])
	if got["computedHash"] != tamperedHash {
		t.Errorf("computedHash = %v, want %v", got["computedHash"], tamperedHash)
	}
	if got["expectedHash"] != genuineHash {
		t.Errorf("expectedHash = %v, want the hash on record %v", got["expectedHash"], genuineHash)
	}
	if got["expectedHash"] == got["computedHash"] {
		t.Error("a tampered file must not report matching hashes")
	}
}

// The QR path: an id, no file, so no computed hash.
func TestVerifyByIDNeedsNoFile(t *testing.T) {
	s, _ := world(t)

	for _, tc := range []struct{ id, want string }{
		{idValid, "VALID"},
		{idRevoked, "REVOKED"},
		{idNotExist, "NOT_ISSUED"},
		// Scanning the QR on the outdated copy leads to the live version.
		{idBirthV1, "VALID"},
	} {
		resp := map[string]any{"expectedHash": zeroHash, "docId": tc.id}
		if err := s.resolve(context.Background(), tc.id, "", resp); err != nil {
			t.Fatalf("resolve %s: %v", tc.id, err)
		}
		if resp["status"] != tc.want {
			t.Errorf("verify by id %s = %v, want %v", tc.id, resp["status"], tc.want)
		}
		if _, present := resp["computedHash"]; present {
			t.Errorf("verify by id %s should not report a computed hash", tc.id)
		}
	}
}
