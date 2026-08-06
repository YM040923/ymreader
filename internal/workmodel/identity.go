package workmodel

import (
	"crypto/sha1"
	"fmt"
	"path"
	"strings"
)

// NormalizePath canonicalizes a Work root path for identity and persistence.
func NormalizePath(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	value = strings.Trim(value, "/")
	cleaned := path.Clean(value)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return ""
	}
	return cleaned
}

// StableWorkID returns the deterministic identifier used by the runtime Work
// builder and the persistent LogicalWork store.
func StableWorkID(libraryID, root string) string {
	sum := sha1.Sum([]byte("work\x00" + libraryID + "\x00" + NormalizePath(root)))
	return fmt.Sprintf("work_%x", sum[:10])
}
