package main

import (
	"bytes"
	"fmt"
	"strings"
)

// renderPDF writes a minimal, genuinely valid single-page PDF. Enough to open
// in a viewer and to have stable bytes — which is all the hashing cares about.
func renderPDF(title string, lines []string) []byte {
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

// escape protects the delimiters that would otherwise break a PDF string.
func escape(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `(`, `\(`, `)`, `\)`)
	return r.Replace(s)
}
