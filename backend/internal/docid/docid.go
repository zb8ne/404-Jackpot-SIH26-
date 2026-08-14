// Package docid carries the marker that ties a PDF back to its registry entry.
//
// Every issued PDF contains, in its content stream, a line reading
//
//	CREDREG-DOCID:<docId>
//
// The QR code printed next to it encodes the same id and is there for the human
// story — scan it with a phone, land on the verify page. The marker is the part
// the backend actually reads, so verification never depends on decoding an image.
package docid

import (
	"bytes"
	"regexp"
)

// Marker is the exact prefix embedded in every issued document.
const Marker = "CREDREG-DOCID:"

// Ids are drawn from a deliberately narrow alphabet so the marker can be found
// in raw bytes without parsing PDF structure.
var pattern = regexp.MustCompile(regexp.QuoteMeta(Marker) + `([A-Za-z0-9][A-Za-z0-9\-_]*)`)

// Extract scans raw file bytes for the marker and returns the id it carries.
// ok is false when the file has no marker at all, which means it did not come
// from this registry.
//
// The last marker wins. Stamps are appended, so if a file somehow carries more
// than one, the newest is the one that describes it.
func Extract(raw []byte) (id string, ok bool) {
	m := pattern.FindAllSubmatch(raw, -1)
	if len(m) == 0 {
		return "", false
	}
	return string(bytes.TrimSpace(m[len(m)-1][1])), true
}
