package pdfdoc

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
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
	downloadURL := "http://127.0.0.1:5173/qr-download?docId=BC-2026-ABC123"
	stamped, visible := StampWithQRDownload(orig, "BC-2026-ABC123", verificationURL, downloadURL)
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
	if !bytes.Contains(stamped, []byte("/Subtype /FileAttachment")) ||
		!bytes.Contains(stamped, []byte("BC-2026-ABC123-qr.png")) ||
		!bytes.Contains(stamped, []byte("\x89PNG\r\n\x1a\n")) {
		t.Fatal("registry page did not embed a downloadable QR PNG attachment")
	}
	if !bytes.Contains(stamped, []byte("/Subtype /Link")) || !bytes.Contains(stamped, []byte(escape(downloadURL))) {
		t.Fatal("registry page did not include the browser-compatible QR download link")
	}
}

func TestQRAttachmentFilenameIsSafe(t *testing.T) {
	if got := safeAttachmentName("BC/2026 (A)"); got != "BC-2026--A-" {
		t.Fatalf("safe attachment name=%q", got)
	}
}

func TestQRAttachmentIsRecognizedByPDFTools(t *testing.T) {
	if _, err := exec.LookPath("pdfdetach"); err != nil {
		t.Skip("pdfdetach is not installed")
	}
	stamped, visible := StampWithQR(Render("BIRTH CERTIFICATE", []string{"Name: Citizen"}), "BC-QR-1", "http://example.test/verify?docId=BC-QR-1")
	if !visible {
		t.Fatal("expected visible registry page")
	}
	dir := t.TempDir()
	pdfPath := filepath.Join(dir, "credential.pdf")
	if err := os.WriteFile(pdfPath, stamped, 0o600); err != nil {
		t.Fatal(err)
	}
	listing, err := exec.Command("pdfdetach", "-list", pdfPath).CombinedOutput()
	if err != nil || !bytes.Contains(listing, []byte("BC-QR-1-qr.png")) {
		t.Fatalf("pdfdetach did not recognize attachment: err=%v output=%s", err, listing)
	}
	if output, err := exec.Command("pdfdetach", "-saveall", "-o", dir, pdfPath).CombinedOutput(); err != nil {
		t.Fatalf("extract attachment: %v: %s", err, output)
	}
	png, err := os.ReadFile(filepath.Join(dir, "BC-QR-1-qr.png"))
	if err != nil || !bytes.HasPrefix(png, []byte("\x89PNG\r\n\x1a\n")) {
		t.Fatalf("extracted attachment is not a PNG: err=%v", err)
	}
}

func TestLegacyBareIDStampingRemainsCompatible(t *testing.T) {
	orig := Render("LICENCE", []string{"Name: Citizen"})
	stamped, _ := Stamp(orig, "DL-LEGACY-1")
	if id, ok := docid.Extract(stamped); !ok || id != "DL-LEGACY-1" {
		t.Fatalf("legacy marker=%q found=%v", id, ok)
	}
}
