package service

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nowen-reader/nowen-reader/internal/model"
	"github.com/nowen-reader/nowen-reader/internal/store"
)

// LibraryRootClaim describes the library that owns a configured root path.
type LibraryRootClaim struct {
	LibraryID   string `json:"libraryId"`
	LibraryName string `json:"libraryName"`
	LibraryType string `json:"libraryType"`
	RootPath    string `json:"rootPath"`
	Canonical   string `json:"-"`
}

// LibraryRootConflict represents an exact physical root shared by libraries.
// Parent/child roots are valid: the deepest matching root owns the content.
type LibraryRootConflict struct {
	Path             string `json:"path"`
	LibraryID        string `json:"libraryId"`
	LibraryName      string `json:"libraryName"`
	OtherLibraryID   string `json:"otherLibraryId"`
	OtherLibraryName string `json:"otherLibraryName"`
}

// LibraryOwnership resolves every physical path to at most one library.
type LibraryOwnership struct {
	claims        []LibraryRootClaim
	exactConflict map[string]bool
	exactOwner    map[string]string
}

// LoadLibraryOwnership builds ownership from every configured library. Disabled
// libraries still reserve their paths, so a parent library cannot expose their
// content under different permissions.
func LoadLibraryOwnership() (*LibraryOwnership, error) {
	libraries, err := store.GetAllLibraries()
	if err != nil {
		return nil, err
	}
	return NewLibraryOwnership(libraries), nil
}

func NewLibraryOwnership(libraries []model.Library) *LibraryOwnership {
	return NewLibraryOwnershipWithRootOwners(libraries, nil)
}

// NewLibraryOwnershipWithRootOwners applies explicit administrator choices to
// otherwise ambiguous exact-root conflicts. Overrides are used only by the
// reconciliation workflow; scanners continue to pause conflicted roots.
func NewLibraryOwnershipWithRootOwners(libraries []model.Library, rootOwners map[string]string) *LibraryOwnership {
	o := &LibraryOwnership{
		exactConflict: make(map[string]bool),
		exactOwner:    make(map[string]string),
	}
	canonicalOwners := make(map[string]string)
	seenClaim := make(map[string]bool)

	for _, lib := range libraries {
		paths := libraryRootPaths(lib)
		for _, rootPath := range paths {
			canonical := canonicalPath(rootPath)
			if canonical == "" {
				continue
			}
			claimKey := lib.ID + "\x00" + canonical
			if seenClaim[claimKey] {
				continue
			}
			seenClaim[claimKey] = true
			if ownerID, ok := canonicalOwners[canonical]; ok && ownerID != lib.ID {
				o.exactConflict[canonical] = true
			} else {
				canonicalOwners[canonical] = lib.ID
			}
			o.claims = append(o.claims, LibraryRootClaim{
				LibraryID:   lib.ID,
				LibraryName: lib.Name,
				LibraryType: lib.Type,
				RootPath:    filepath.Clean(rootPath),
				Canonical:   canonical,
			})
		}
	}

	// Deepest roots are checked first, which makes child-library ownership
	// deterministic even when a parent library also scans the containing tree.
	sort.SliceStable(o.claims, func(i, j int) bool {
		return len(o.claims[i].Canonical) > len(o.claims[j].Canonical)
	})
	for path, libraryID := range rootOwners {
		canonical := canonicalPath(path)
		if !o.exactConflict[canonical] {
			continue
		}
		for _, claim := range o.claims {
			if claim.Canonical == canonical && claim.LibraryID == libraryID {
				o.exactOwner[canonical] = libraryID
				break
			}
		}
	}
	return o
}

func libraryRootPaths(lib model.Library) []string {
	paths := lib.RootPaths
	if len(paths) == 0 {
		paths = []string{lib.RootPath}
	}
	return paths
}

// OwnerForPath returns the deepest matching root. Exact roots shared by two
// libraries are intentionally unresolved until an administrator fixes them.
func (o *LibraryOwnership) OwnerForPath(path string) (LibraryRootClaim, bool) {
	canonical := canonicalPath(path)
	if canonical == "" {
		return LibraryRootClaim{}, false
	}
	return o.ownerForCanonicalPath(canonical)
}

func (o *LibraryOwnership) ownerForCanonicalPath(canonical string) (LibraryRootClaim, bool) {
	bestDepth := -1
	var best LibraryRootClaim
	ambiguous := false
	for _, claim := range o.claims {
		if !pathWithin(claim.Canonical, canonical) {
			continue
		}
		if preferred := o.exactOwner[claim.Canonical]; preferred != "" && claim.LibraryID != preferred {
			continue
		}
		depth := len(claim.Canonical)
		if depth < bestDepth {
			break
		}
		if depth > bestDepth {
			best = claim
			bestDepth = depth
			ambiguous = o.exactConflict[claim.Canonical] && o.exactOwner[claim.Canonical] == ""
			continue
		}
		if claim.LibraryID != best.LibraryID {
			ambiguous = true
		}
	}
	if bestDepth < 0 || ambiguous {
		return LibraryRootClaim{}, false
	}
	return best, true
}

func (o *LibraryOwnership) IsOwnedBy(libraryID, path string) bool {
	owner, ok := o.OwnerForPath(path)
	return ok && owner.LibraryID == libraryID
}

func (o *LibraryOwnership) isCanonicalPathOwnedBy(libraryID, canonicalPath string) bool {
	owner, ok := o.ownerForCanonicalPath(canonicalPath)
	return ok && owner.LibraryID == libraryID
}

func (o *LibraryOwnership) RootHasExactConflict(rootPath string) bool {
	return o.exactConflict[canonicalPath(rootPath)]
}

func (o *LibraryOwnership) RootConflictIsResolved(rootPath string) bool {
	canonical := canonicalPath(rootPath)
	return o.exactConflict[canonical] && o.exactOwner[canonical] != ""
}

func (o *LibraryOwnership) ExactRootConflicts() []LibraryRootConflict {
	claimsByPath := make(map[string][]LibraryRootClaim)
	for _, claim := range o.claims {
		if o.exactConflict[claim.Canonical] {
			claimsByPath[claim.Canonical] = append(claimsByPath[claim.Canonical], claim)
		}
	}
	var conflicts []LibraryRootConflict
	for _, claims := range claimsByPath {
		for i := 0; i < len(claims); i++ {
			for j := i + 1; j < len(claims); j++ {
				if claims[i].LibraryID == claims[j].LibraryID {
					continue
				}
				conflicts = append(conflicts, LibraryRootConflict{
					Path:             claims[i].RootPath,
					LibraryID:        claims[i].LibraryID,
					LibraryName:      claims[i].LibraryName,
					OtherLibraryID:   claims[j].LibraryID,
					OtherLibraryName: claims[j].LibraryName,
				})
			}
		}
	}
	sort.Slice(conflicts, func(i, j int) bool {
		if conflicts[i].Path != conflicts[j].Path {
			return conflicts[i].Path < conflicts[j].Path
		}
		if conflicts[i].LibraryID != conflicts[j].LibraryID {
			return conflicts[i].LibraryID < conflicts[j].LibraryID
		}
		return conflicts[i].OtherLibraryID < conflicts[j].OtherLibraryID
	})
	return conflicts
}

// RelativePathForOwner returns the path relative to the winning root.
func (o *LibraryOwnership) RelativePathForOwner(path string) (LibraryRootClaim, string, bool) {
	owner, ok := o.OwnerForPath(path)
	if !ok {
		return LibraryRootClaim{}, "", false
	}
	rel, err := filepath.Rel(owner.Canonical, canonicalPath(path))
	if err != nil || rel == "." || rel == "" || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return LibraryRootClaim{}, "", false
	}
	return owner, filepath.ToSlash(rel), true
}

// ValidateLibraryRootUniqueness rejects only exact roots owned by another
// library. Parent/child relationships are supported by deepest-root ownership.
func ValidateLibraryRootUniqueness(libraryID string, paths []string) ([]LibraryRootConflict, error) {
	libraries, err := store.GetAllLibraries()
	if err != nil {
		return nil, err
	}

	requested := make(map[string]string)
	for _, path := range paths {
		if canonical := canonicalPath(path); canonical != "" {
			requested[canonical] = filepath.Clean(path)
		}
	}

	var conflicts []LibraryRootConflict
	for _, lib := range libraries {
		if lib.ID == libraryID {
			continue
		}
		for _, otherPath := range libraryRootPaths(lib) {
			canonical := canonicalPath(otherPath)
			path, found := requested[canonical]
			if !found {
				continue
			}
			conflicts = append(conflicts, LibraryRootConflict{
				Path:             path,
				LibraryID:        libraryID,
				OtherLibraryID:   lib.ID,
				OtherLibraryName: lib.Name,
			})
		}
	}
	return conflicts, nil
}

func canonicalPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	cleaned := filepath.Clean(path)
	abs, err := filepath.Abs(cleaned)
	if err == nil {
		cleaned = abs
	}
	// Resolve the longest existing ancestor as well. This keeps comparisons
	// stable on systems where /var and /private/var refer to the same location,
	// even when the final file or directory has not been created yet.
	for probe := cleaned; ; probe = filepath.Dir(probe) {
		if _, err := os.Stat(probe); err == nil {
			if resolved, err := filepath.EvalSymlinks(probe); err == nil {
				if suffix, relErr := filepath.Rel(probe, cleaned); relErr == nil && suffix != "." {
					cleaned = filepath.Join(resolved, suffix)
				} else {
					cleaned = resolved
				}
			}
			break
		}
		parent := filepath.Dir(probe)
		if parent == probe {
			break
		}
	}
	return filepath.Clean(cleaned)
}

func pathWithin(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
