package handler

import (
	"net/http"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nowen-reader/nowen-reader/internal/service"
	"github.com/nowen-reader/nowen-reader/internal/store"
)

const (
	defaultWorkDirectoryPageSize = 100
	maxWorkDirectoryPageSize     = 500
)

type workDirectoryBreadcrumb struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type workDirectoryNode struct {
	Name         string `json:"name"`
	Path         string `json:"path"`
	WorkCount    int    `json:"workCount"`
	HasChildren  bool   `json:"hasChildren"`
	CoverURL     string `json:"coverUrl,omitempty"`
	CoverComicID string `json:"coverComicId,omitempty"`
}

type workDirectoryListing struct {
	LibraryID   string                    `json:"libraryId"`
	Parent      string                    `json:"parent"`
	Breadcrumbs []workDirectoryBreadcrumb `json:"breadcrumbs"`
	Directories []workDirectoryNode       `json:"directories"`
	Total       int                       `json:"total"`
	Page        int                       `json:"page"`
	PageSize    int                       `json:"pageSize"`
	TotalPages  int                       `json:"totalPages"`
}

// normalizeWorkDirectory converts an API/Work relative path into the canonical
// slash-separated form used by Work.RootPath.
func normalizeWorkDirectory(value string) string {
	return strings.Trim(strings.ReplaceAll(strings.TrimSpace(value), "\\", "/"), "/")
}

func validWorkDirectory(value string) bool {
	if value == "" {
		return true
	}
	if strings.ContainsRune(value, '\\') {
		return false
	}
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
	}
	return true
}

// workDirectoryPath is the directory a Work belongs to in browse mode. A Work
// backed by a single loose file stores the file stem in RootPath, so its real
// directory is the parent path; otherwise RootPath is already the directory.
func workDirectoryPath(work service.Work) string {
	root := normalizeWorkDirectory(work.RootPath)
	if root == "" {
		return ""
	}
	if len(work.Units) == 1 {
		relative := normalizeWorkDirectory(work.Units[0].RelativePath)
		if relative != "" {
			stem := strings.TrimSuffix(relative, path.Ext(relative))
			if strings.EqualFold(stem, root) {
				if index := strings.LastIndex(root, "/"); index >= 0 {
					return root[:index]
				}
				return ""
			}
		}
	}
	return root
}

// buildWorkDirectoryNodes derives immediate child directories below parent
// from the Work root paths. Works whose root path equals parent are shown as
// cards by the works endpoint and are not returned as directories.
func buildWorkDirectoryNodes(works []service.Work, parent string) []workDirectoryNode {
	nodes := make(map[string]*workDirectoryNode)
	for _, work := range works {
		root := workDirectoryPath(work)
		if root == "" {
			continue
		}
		var childPath string
		if parent == "" {
			childPath = strings.SplitN(root, "/", 2)[0]
		} else {
			if root == parent {
				continue
			}
			prefix := parent + "/"
			if !strings.HasPrefix(root, prefix) {
				continue
			}
			rest := strings.TrimPrefix(root, prefix)
			if rest == "" {
				continue
			}
			childPath = prefix + strings.SplitN(rest, "/", 2)[0]
		}
		node, ok := nodes[childPath]
		if !ok {
			node = &workDirectoryNode{
				Name: strings.TrimPrefix(childPath, parent+"/"),
				Path: childPath,
			}
			if parent == "" {
				node.Name = childPath
			}
			nodes[childPath] = node
		}
		node.WorkCount++
		if parent == "" {
			if strings.Contains(root, "/") {
				node.HasChildren = true
			}
		} else if strings.HasPrefix(root, childPath+"/") {
			node.HasChildren = true
		}
		if node.CoverURL == "" && strings.TrimSpace(work.CoverURL) != "" {
			node.CoverURL = work.CoverURL
		}
		if node.CoverComicID == "" && strings.TrimSpace(work.CoverComicID) != "" {
			node.CoverComicID = work.CoverComicID
		}
	}
	result := make([]workDirectoryNode, 0, len(nodes))
	for _, node := range nodes {
		result = append(result, *node)
	}
	sort.SliceStable(result, func(i, j int) bool {
		return naturalWorkLess(result[i].Name, result[j].Name)
	})
	return result
}

// ListWorkDirectories returns the immediate classification subdirectories of
// one library for the optional directory browsing mode.
// GET /api/libraries/:id/directories?parent=&page=&pageSize=
func (h *LibraryHandler) ListWorkDirectories(c *gin.Context) {
	uid := getUserID(c)
	if uid == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Login required"})
		return
	}
	libraryID := strings.TrimSpace(c.Param("id"))
	if libraryID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Library ID is required"})
		return
	}
	allowed, err := store.UserCanViewLibrary(uid, libraryID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check library permission"})
		return
	}
	if !allowed {
		c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden: no access to this library"})
		return
	}

	rawParent := strings.TrimSpace(c.Query("parent"))
	if !validWorkDirectory(rawParent) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid directory path"})
		return
	}
	parent := normalizeWorkDirectory(rawParent)

	page := 1
	if raw := strings.TrimSpace(c.Query("page")); raw != "" {
		parsed, parseErr := strconv.Atoi(raw)
		if parseErr != nil || parsed < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "page must be a positive integer"})
			return
		}
		page = parsed
	}
	pageSize := defaultWorkDirectoryPageSize
	if raw := strings.TrimSpace(c.Query("pageSize")); raw != "" {
		parsed, parseErr := strconv.Atoi(raw)
		if parseErr != nil || parsed < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "pageSize must be a positive integer"})
			return
		}
		pageSize = parsed
	}
	if pageSize > maxWorkDirectoryPageSize {
		pageSize = maxWorkDirectoryPageSize
	}

	catalog, err := NewWorkHandler().loadWorkCatalog(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load works"})
		return
	}
	libraryWorks := make([]service.Work, 0, len(catalog.Works))
	for _, work := range catalog.Works {
		if work.LibraryID == libraryID {
			libraryWorks = append(libraryWorks, work)
		}
	}

	directories := buildWorkDirectoryNodes(libraryWorks, parent)
	total := len(directories)
	totalPages := 1
	if total > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}
	start := (page - 1) * pageSize
	if start >= total {
		directories = []workDirectoryNode{}
	} else {
		end := start + pageSize
		if end > total {
			end = total
		}
		directories = directories[start:end]
	}

	breadcrumbs := make([]workDirectoryBreadcrumb, 0, 4)
	if parent != "" {
		segments := strings.Split(parent, "/")
		for index, segment := range segments {
			breadcrumbs = append(breadcrumbs, workDirectoryBreadcrumb{
				Name: segment,
				Path: strings.Join(segments[:index+1], "/"),
			})
		}
	}

	c.Header("Cache-Control", "private, max-age=15, stale-while-revalidate=60")
	c.JSON(http.StatusOK, workDirectoryListing{
		LibraryID:   libraryID,
		Parent:      parent,
		Breadcrumbs: breadcrumbs,
		Directories: directories,
		Total:       total,
		Page:        page,
		PageSize:    pageSize,
		TotalPages:  totalPages,
	})
}
