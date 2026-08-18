package pdfdoc

import (
	"bytes"
	"testing"

	"credreg/backend/internal/docid"
)

func TestStampAddsAFindableMarker(t *testing.T) {
	orig := Render("CERTIFICATE OF BIRTH", []string{"Name: Asha Menon"})

	if _, ok := docid.Extract(orig); ok {
		t.Fatal("an unstamped document should carry no marker")
	}

	stamped, visible := Stamp(orig, "BC-2019-004471")
	if !visible {
		t.Error("a PDF we generated ourselves should get a real registry page")
	}

	id, ok := docid.Extract(stamped)
	if !ok || id != "BC-2019-004471" {
		t.Fatalf("extracted %q, %v; want BC-2019-004471", id, ok)
	}
}

// The original bytes must survive untouched: stamping appends, it never rewrites.
func TestStampIsAnIncrementalUpdate(t *testing.T) {
	orig := Render("DRIVING LICENCE", []string{"Name: Rahul Iyer"})
	stamped, _ := Stamp(orig, "DL-GA-2016-33017")

	if !bytes.HasPrefix(stamped, orig) {
		t.Error("stamping must leave the original bytes byte-for-byte intact")
	}
	if len(stamped) <= len(orig) {
		t.Error("stamping should append content")
	}
}

func TestStampedPDFStaysWellFormed(t *testing.T) {
	stamped, _ := Stamp(Render("DEGREE", []string{"Name: Asha Menon"}), "DEG-2021-1174")

	// A second page, a chained xref, and a terminating EOF.
	if got := bytes.Count(stamped, []byte("/Type /Page\n")) + bytes.Count(stamped, []byte("/Type /Page ")); got < 2 {
		t.Errorf("expected the original page plus a registry page, found %d", got)
	}
	if !bytes.Contains(stamped, []byte("/Prev ")) {
		t.Error("the appended xref must chain to the original one")
	}
	if !bytes.HasSuffix(bytes.TrimSpace(stamped), []byte("%%EOF")) {
		t.Error("a PDF must end with an end-of-file marker")
	}
	if bytes.Count(stamped, []byte("/Count 2")) != 1 {
		t.Error("the page tree should report two pages")
	}
}

// Anything this parser cannot follow still has to come back verifiable.
func TestStampFallsBackForUnparseablePDFs(t *testing.T) {
	junk := []byte("%PDF-1.7\nthis is not a cross-reference table anyone can follow\n")

	stamped, visible := Stamp(junk, "BC-9999-999999")
	if visible {
		t.Error("expected the fallback path for an unparseable PDF")
	}
	id, ok := docid.Extract(stamped)
	if !ok || id != "BC-9999-999999" {
		t.Fatalf("fallback must still embed a findable marker, got %q, %v", id, ok)
	}
	if !bytes.HasPrefix(stamped, junk) {
		t.Error("the fallback must also leave the original bytes intact")
	}
}

// Stamping is deterministic: the same document and id give the same bytes, so
// the anchored hash is reproducible.
func TestStampIsDeterministic(t *testing.T) {
	orig := Render("DEGREE", []string{"Name: Asha Menon"})
	a, _ := Stamp(orig, "DEG-2021-1174")
	b, _ := Stamp(orig, "DEG-2021-1174")
	if !bytes.Equal(a, b) {
		t.Error("stamping the same bytes twice must produce identical output")
	}
}

func TestStampWithConfiguredVerificationURLPreservesMarker(t *testing.T) {
	orig := Render("BIRTH CERTIFICATE", []string{"Name: Citizen"})
	verificationURL := "http://127.0.0.1:5173/verify?docId=BC-2026-ABC123"
	stamped, visible := StampWithQR(orig, "BC-2026-ABC123", verificationURL)
	if !visible {
		t.Fatal("expected visible registry page")
	}
	if id, ok := docid.Extract(stamped); !ok || id != "BC-2026-ABC123" {
		t.Fatalf("marker=%q found=%v", id, ok)
	}
	bare, _ := Stamp(orig, "BC-2026-ABC123")
	if bytes.Equal(stamped, bare) {
		t.Fatal("URL QR payload must differ from legacy bare-ID QR payload")
	}
	if renderQR(verificationURL, 0, 0, 100) == renderQR("BC-2026-ABC123", 0, 0, 100) {
		t.Fatal("QR rendering ignored configured payload")
	}
}

func TestLegacyBareIDStampingRemainsCompatible(t *testing.T) {
	orig := Render("LICENCE", []string{"Name: Citizen"})
	stamped, _ := Stamp(orig, "DL-LEGACY-1")
	if id, ok := docid.Extract(stamped); !ok || id != "DL-LEGACY-1" {
		t.Fatalf("legacy marker=%q found=%v", id, ok)
	}
}
