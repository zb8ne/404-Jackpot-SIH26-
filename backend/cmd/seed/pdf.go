package main

import (
	"bytes"
	"fmt"
	"strings"

	qrcode "github.com/skip2/go-qrcode"

	"credreg/backend/internal/docid"
)

// renderPDF writes a minimal, genuinely valid single-page PDF carrying two
// copies of its identity: a QR code for the audience to scan, and the
// CREDREG-DOCID marker the backend actually reads. The content stream is left
// uncompressed so the marker is findable in the raw bytes.
func renderPDF(title string, lines []string, docID string) []byte {
	var content bytes.Buffer

	content.WriteString("BT\n/F1 18 Tf\n72 720 Td\n")
	fmt.Fprintf(&content, "(%s) Tj\n", escape(title))
	content.WriteString("/F1 12 Tf\n")
	for _, l := range lines {
		fmt.Fprintf(&content, "0 -28 Td\n(%s) Tj\n", escape(l))
	}
	content.WriteString("0 -56 Td\n")
	fmt.Fprintf(&content, "(%s) Tj\n", escape("This document is anchored on the government credential registry."))
	content.WriteString("ET\n")

	content.WriteString(renderQR(docID, 396, 560, 144))

	// The marker, drawn small under the QR. Both the visible text and the bytes
	// in this stream read exactly CREDREG-DOCID:<docId>.
	content.WriteString("BT\n/F1 8 Tf\n396 544 Td\n")
	fmt.Fprintf(&content, "(%s%s) Tj\nET\n", docid.Marker, escape(docID))

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

// renderQR draws a QR code as vector rectangles, one per dark module. Drawing it
// rather than embedding an image keeps the PDF to five objects and no filters.
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
