// Package pdfdoc renders demo documents and stamps issued ones.
//
// Stamping is the important half. A document only becomes verifiable once it
// carries its registry id, so /issue stamps an uploaded PDF and anchors the hash
// of the stamped bytes — the version the citizen actually holds.
package pdfdoc

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	qrcode "github.com/skip2/go-qrcode"

	"credreg/backend/internal/docid"
)

// Render writes a minimal, genuinely valid single-page PDF. It carries no
// registry marks of its own: those are added by Stamp when the document is
// issued, so there is one stamping path rather than two.
func Render(title string, lines []string) []byte {
	var content bytes.Buffer
	content.WriteString("BT\n/F1 18 Tf\n72 720 Td\n")
	fmt.Fprintf(&content, "(%s) Tj\n", escape(title))
	content.WriteString("/F1 12 Tf\n")
	for _, l := range lines {
		fmt.Fprintf(&content, "0 -28 Td\n(%s) Tj\n", escape(l))
	}
	content.WriteString("ET\n")
	stream := content.String()

	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] " +
			"/Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream), stream),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
	}

	var out bytes.Buffer
	out.WriteString("%PDF-1.4\n")

	offsets := make([]int, len(objects))
	for i, obj := range objects {
		offsets[i] = out.Len()
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", i+1, obj)
	}

	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n", len(objects)+1)
	out.WriteString("0000000000 65535 f \n")
	for _, off := range offsets {
		fmt.Fprintf(&out, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n",
		len(objects)+1, xref)

	return out.Bytes()
}

var (
	reObjNum    = regexp.MustCompile(`(?m)^(\d+)\s+0\s+obj\b`)
	reStartXref = regexp.MustCompile(`startxref\s+(\d+)`)
	reRoot      = regexp.MustCompile(`/Root\s+(\d+)\s+0\s+R`)
	reKids      = regexp.MustCompile(`/Kids\s*\[([^\]]*)\]`)
	reCount     = regexp.MustCompile(`/Count\s+(\d+)`)
)

// Stamp adds a registry page to an existing PDF: a QR code encoding the docId,
// the CREDREG-DOCID marker, and a line of explanation.
//
// It is written as an incremental update — the original bytes are left byte-for-
// byte intact and everything new is appended — so stamping never disturbs the
// document a department uploaded.
//
// visible reports whether the registry page was actually added. For a PDF this
// simple parser cannot follow (compressed cross-reference streams, mostly) it
// falls back to appending the marker as a trailing comment: still verifiable,
// but with no page to look at.
func Stamp(orig []byte, docID string) (stamped []byte, visible bool) {
	return StampWithQR(orig, docID, docID)
}

// StampWithQR preserves the embedded document marker while allowing newly
// issued credentials to carry an explicit verification URL in their QR code.
// Stamp remains the compatibility path for older bare-document-ID QR values.
func StampWithQR(orig []byte, docID, qrPayload string) (stamped []byte, visible bool) {
	out, err := appendRegistryPage(orig, docID, qrPayload)
	if err != nil {
		return appendMarkerComment(orig, docID), false
	}
	return out, true
}

// appendMarkerComment is the fallback: a PDF comment after the final %%EOF.
// Viewers ignore trailing bytes, and the marker is still there in the raw file.
func appendMarkerComment(orig []byte, docID string) []byte {
	out := make([]byte, 0, len(orig)+len(docID)+32)
	out = append(out, orig...)
	if !bytes.HasSuffix(out, []byte("\n")) {
		out = append(out, '\n')
	}
	return append(out, []byte("%"+docid.Marker+docID+"\n")...)
}

func appendRegistryPage(orig []byte, docID, qrPayload string) ([]byte, error) {
	// Where the current cross-reference table starts; the new one chains to it.
	sx := reStartXref.FindAllSubmatch(orig, -1)
	if len(sx) == 0 {
		return nil, fmt.Errorf("no startxref")
	}
	prevXref, err := strconv.Atoi(string(sx[len(sx)-1][1]))
	if err != nil {
		return nil, err
	}
	// A classic trailer keyword means a classic xref table, which is the only
	// layout this parser handles.
	if !bytes.Contains(orig, []byte("trailer")) {
		return nil, fmt.Errorf("no classic trailer")
	}

	roots := reRoot.FindAllSubmatch(orig, -1)
	if len(roots) == 0 {
		return nil, fmt.Errorf("no /Root")
	}
	rootNum := string(roots[len(roots)-1][1])

	catalog, err := findObject(orig, rootNum)
	if err != nil {
		return nil, err
	}
	pagesRef := regexp.MustCompile(`/Pages\s+(\d+)\s+0\s+R`).FindStringSubmatch(catalog)
	if pagesRef == nil {
		return nil, fmt.Errorf("catalog has no /Pages")
	}
	pagesNum := pagesRef[1]

	pagesBody, err := findObject(orig, pagesNum)
	if err != nil {
		return nil, err
	}
	kids := reKids.FindStringSubmatchIndex(pagesBody)
	count := reCount.FindStringSubmatchIndex(pagesBody)
	if kids == nil || count == nil {
		return nil, fmt.Errorf("page tree has no /Kids or /Count")
	}
	oldCount, err := strconv.Atoi(pagesBody[count[2]:count[3]])
	if err != nil {
		return nil, err
	}

	// New object numbers follow the highest one already in the file.
	maxObj := 0
	for _, m := range reObjNum.FindAllSubmatch(orig, -1) {
		if n, err := strconv.Atoi(string(m[1])); err == nil && n > maxObj {
			maxObj = n
		}
	}
	if maxObj == 0 {
		return nil, fmt.Errorf("no objects found")
	}
	fontNum, contentNum, pageNum := maxObj+1, maxObj+2, maxObj+3

	// Rewrite the page tree in place, keeping every other key it carries.
	newKids := strings.TrimSpace(pagesBody[kids[2]:kids[3]]) + fmt.Sprintf(" %d 0 R", pageNum)
	updatedPages := pagesBody[:kids[2]] + " " + newKids + " " + pagesBody[kids[3]:]
	shift := len(updatedPages) - len(pagesBody)
	updatedPages = updatedPages[:count[2]+shift] +
		strconv.Itoa(oldCount+1) + updatedPages[count[3]+shift:]

	stream := registryPageContent(docID, qrPayload)

	var out bytes.Buffer
	out.Write(orig)
	if !bytes.HasSuffix(orig, []byte("\n")) {
		out.WriteByte('\n')
	}

	type entry struct {
		num    int
		offset int
	}
	var entries []entry

	write := func(num int, body string) {
		entries = append(entries, entry{num: num, offset: out.Len()})
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", num, body)
	}

	pagesNumInt, err := strconv.Atoi(pagesNum)
	if err != nil {
		return nil, err
	}
	write(pagesNumInt, updatedPages)
	write(fontNum, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")
	write(contentNum, fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream), stream))
	write(pageNum, fmt.Sprintf(
		"<< /Type /Page /Parent %s 0 R /MediaBox [0 0 612 792] "+
			"/Resources << /Font << /F1 %d 0 R >> >> /Contents %d 0 R >>",
		pagesNum, fontNum, contentNum))

	// Cross-reference entries must be written in ascending object order, grouped
	// into contiguous subsections.
	xrefOffset := out.Len()
	out.WriteString("xref\n")
	for i := 0; i < len(entries); {
		j := i + 1
		for j < len(entries) && entries[j].num == entries[j-1].num+1 {
			j++
		}
		fmt.Fprintf(&out, "%d %d\n", entries[i].num, j-i)
		for _, e := range entries[i:j] {
			fmt.Fprintf(&out, "%010d 00000 n \n", e.offset)
		}
		i = j
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root %s 0 R /Prev %d >>\nstartxref\n%d\n%%%%EOF\n",
		pageNum+1, rootNum, prevXref, xrefOffset)

	return out.Bytes(), nil
}

// findObject returns the body of "<num> 0 obj ... endobj".
func findObject(raw []byte, num string) (string, error) {
	re := regexp.MustCompile(`(?s)\b` + num + `\s+0\s+obj\b(.*?)endobj`)
	m := re.FindSubmatch(raw)
	if m == nil {
		return "", fmt.Errorf("object %s not found", num)
	}
	return strings.TrimSpace(string(m[1])), nil
}

func registryPageContent(docID, qrPayload string) string {
	var b strings.Builder

	b.WriteString("BT\n/F1 16 Tf\n72 720 Td\n(CREDENTIAL REGISTRY ANCHOR) Tj\nET\n")
	b.WriteString("BT\n/F1 10 Tf\n72 700 Td\n")
	fmt.Fprintf(&b, "(%s) Tj\nET\n",
		escape("This document is anchored on the government credential registry."))
	b.WriteString("BT\n/F1 10 Tf\n72 686 Td\n")
	fmt.Fprintf(&b, "(%s) Tj\nET\n",
		escape("Scan the code, or upload this file, to check that it is authentic and current."))

	b.WriteString(renderQR(qrPayload, 72, 480, 180))

	b.WriteString("BT\n/F1 11 Tf\n72 458 Td\n")
	fmt.Fprintf(&b, "(%s%s) Tj\nET\n", docid.Marker, escape(docID))

	return b.String()
}

// renderQR draws a QR code as vector rectangles, one per dark module. Drawing it
// rather than embedding an image keeps the appended objects simple and filter-free.
func renderQR(text string, x, y, size float64) string {
	q, err := qrcode.New(text, qrcode.Medium)
	if err != nil {
		return ""
	}
	bitmap := q.Bitmap()
	n := len(bitmap)
	if n == 0 {
		return ""
	}
	module := size / float64(n)

	var b strings.Builder
	b.WriteString("q\n1 1 1 rg\n")
	fmt.Fprintf(&b, "%.2f %.2f %.2f %.2f re f\n", x, y, size, size)
	b.WriteString("0 0 0 rg\n")
	for row := range bitmap {
		for col := range bitmap[row] {
			if !bitmap[row][col] {
				continue
			}
			// PDF's origin is bottom-left; the bitmap's is top-left.
			mx := x + float64(col)*module
			my := y + size - float64(row+1)*module
			fmt.Fprintf(&b, "%.2f %.2f %.2f %.2f re f\n", mx, my, module, module)
		}
	}
	b.WriteString("Q\n")
	return b.String()
}

// escape protects the delimiters that would otherwise break a PDF string.
func escape(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `(`, `\(`, `)`, `\)`)
	return r.Replace(s)
}
