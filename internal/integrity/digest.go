// Package integrity computes SHA-256 integrity digests for craft packages.
package integrity

import (
	"crypto/sha256"
	"encoding/base64"
	"sort"
)

// Digest computes a SHA-256 integrity digest from a map of file paths to
// contents. Files are sorted by path, concatenated, then hashed. Returns
// the digest in "sha256-<base64>" format per the RFC specification.
//
// An empty file map produces a digest of the empty string.
func Digest(files map[string][]byte) string {
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	h := sha256.New()
	for _, p := range paths {
		h.Write([]byte(p))
		h.Write([]byte{0})
		h.Write(files[p])
		h.Write([]byte{0})
	}

	return "sha256-" + base64.StdEncoding.EncodeToString(h.Sum(nil))
}
