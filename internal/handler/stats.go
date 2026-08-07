package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nowen-reader/nowen-reader/internal/service"
	"github.com/nowen-reader/nowen-reader/internal/store"
)

// StatsHandler handles reading statistics API endpoints.
type StatsHandler struct{}

type readingActivityRequest struct {
	ClientSessionID string `json:"clientSessionId"`
	Page            int    `json:"page"`
	TotalPages      int    `json:"totalPages"`
	ActiveSeconds   int    `json:"activeSeconds"`
	Sequence        int    `json:"sequence"`
	Finalize        bool   `json:"finalize"`
	TrackProgress   *bool  `json:"trackProgress"`
}

// NewStatsHandler creates a new StatsHandler.
func NewStatsHandler() *StatsHandler {
	return &StatsHandler{}
}

// POST /api/reading/:id/activity — 幂等记录阅读进度与会话心跳。
func (h *StatsHandler) RecordActivity(c *gin.Context) {
	comicID := c.Param("id")
	var body readingActivityRequest
	if err := c.ShouldBindJSON(&body); err != nil || body.ClientSessionID == "" || len(body.ClientSessionID) > 128 || body.Sequence <= 0 || body.ActiveSeconds < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid reading activity"})
		return
	}
	if err := checkComicAccess(c, comicID); err != nil {
		return
	}
	trackProgress := true
	if body.TrackProgress != nil {
		trackProgress = *body.TrackProgress
	}
	if err := store.RecordReadingActivity(
		comicID, getUserID(c), body.ClientSessionID, body.Page, body.TotalPages,
		body.ActiveSeconds, body.Sequence, body.Finalize, trackProgress,
	); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record reading activity"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// GET /api/stats — Get reading statistics
func (h *StatsHandler) GetStats(c *gin.Context) {
	if wantsWorkStats(c) {
		stats, err := h.getWorkReadingAnalytics(c)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get Work stats"})
			return
		}
		c.JSON(http.StatusOK, stats)
		return
	}
	uid := getUserID(c)
	stats, err := store.GetReadingStats(uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get stats"})
		return
	}
	c.JSON(http.StatusOK, stats)
}

// GET /api/stats/yearly?year=2024 — 年度阅读报告
func (h *StatsHandler) GetYearlyReport(c *gin.Context) {
	yearStr := c.DefaultQuery("year", strconv.Itoa(time.Now().Year()))
	year, err := strconv.Atoi(yearStr)
	if err != nil || year < 2000 || year > 2100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid year"})
		return
	}

	if wantsWorkStats(c) {
		works, novels, sessions, loadErr := h.getWorkReadingInputs(c)
		if loadErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get yearly Work report"})
			return
		}
		c.JSON(http.StatusOK, service.AggregateWorkYearlyReport(year, sessions, works, novels))
		return
	}

	report, err := store.GetYearlyReadingReport(year, getUserID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get yearly report"})
		return
	}
	c.JSON(http.StatusOK, report)
}

// POST /api/stats/session — Start reading session
func (h *StatsHandler) StartSession(c *gin.Context) {
	var body struct {
		ComicID   string `json:"comicId"`
		StartPage int    `json:"startPage"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	if body.ComicID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "comicId required"})
		return
	}

	if err := checkComicAccess(c, body.ComicID); err != nil {
		return
	}

	sessionID, err := store.StartReadingSession(body.ComicID, body.StartPage, getUserID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start session"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"sessionId": sessionID})
}

// PUT /api/stats/session — End reading session
func (h *StatsHandler) EndSession(c *gin.Context) {
	var body struct {
		SessionID int `json:"sessionId"`
		EndPage   int `json:"endPage"`
		Duration  int `json:"duration"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	if body.SessionID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sessionId and duration required"})
		return
	}

	// 校验会话关联漫画的书库权限
	if comicID, err := store.GetReadingSessionComicID(body.SessionID); err == nil {
		if err := checkComicAccess(c, comicID); err != nil {
			return
		}
	}

	if err := store.EndReadingSession(body.SessionID, body.EndPage, body.Duration, getUserID(c)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to end session"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// GET /api/stats/enhanced — 增强版阅读统计
func (h *StatsHandler) GetEnhancedStats(c *gin.Context) {
	if wantsWorkStats(c) {
		stats, err := h.getWorkReadingAnalytics(c)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get enhanced Work stats"})
			return
		}
		c.JSON(http.StatusOK, stats)
		return
	}
	stats, err := store.GetEnhancedReadingStats(getUserID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, stats)
}

// GetHistory returns one row per logical Work in Work view, while novels keep
// their physical catalog identity.
func (h *StatsHandler) GetHistory(c *gin.Context) {
	if !wantsWorkStats(c) {
		stats, err := store.GetReadingStats(getUserID(c))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get reading history"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": stats.RecentSessions, "total": len(stats.RecentSessions)})
		return
	}
	stats, err := h.getWorkReadingAnalytics(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get Work history"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": stats.History, "total": len(stats.History)})
}

// DeleteHistory removes reading history for one or more works/comics and resets
// their reading progress. This is a hard delete from the ReadingSession table,
// unlike clearing status which only hides the entry.
func (h *StatsHandler) DeleteHistory(c *gin.Context) {
	var body struct {
		ComicIDs []string `json:"comicIds"`
		WorkIDs  []string `json:"workIds"`
		All      bool     `json:"all"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	uid := getUserID(c)
	if body.All {
		deleted, err := store.DeleteAllReadingHistory(uid)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "deleted": deleted})
		return
	}
	comicIDs := append([]string(nil), body.ComicIDs...)
	if len(body.WorkIDs) > 0 {
		catalog, err := NewWorkHandler().loadWorkCatalog(c)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		for _, workID := range body.WorkIDs {
			work := catalog.ByID[workID]
			if work == nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Work not found"})
				return
			}
			comicIDs = append(comicIDs, service.PhysicalComicIDs(*work)...)
		}
	}
	if len(comicIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "comicIds, workIds or all is required"})
		return
	}
	deleted, err := store.DeleteReadingHistoryByComicIDs(comicIDs, uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "deleted": deleted})
}

func wantsWorkStats(c *gin.Context) bool {
	return strings.EqualFold(strings.TrimSpace(c.Query("view")), "work")
}

func includeNovelsInWorkStats(c *gin.Context) bool {
	return strings.EqualFold(strings.TrimSpace(c.Query("includeNovels")), "true")
}

func (h *StatsHandler) getWorkReadingAnalytics(c *gin.Context) (*service.WorkReadingAnalytics, error) {
	works, novels, sessions, err := h.getWorkReadingInputs(c)
	if err != nil {
		return nil, err
	}
	return service.AggregateWorkReadingAnalytics(sessions, works, novels, time.Now()), nil
}

func (h *StatsHandler) getWorkReadingInputs(c *gin.Context) ([]service.Work, []store.ComicListItem, []store.ReadingSessionRecord, error) {
	works, err := NewWorkHandler().loadWorks(c, false)
	if err != nil {
		return nil, nil, nil, err
	}
	libraryIDs, filterLibraries, err := resolveWorkLibraryScope(c)
	if err != nil {
		return nil, nil, nil, err
	}
	novels := []store.ComicListItem{}
	if includeNovelsInWorkStats(c) {
		result, queryErr := store.GetAllComics(store.ComicListOptions{
			ContentType: "novel", SortBy: "lastReadAt", SortOrder: "desc",
			Page: 0, PageSize: 0, UserID: getUserID(c),
			LibraryIDs: libraryIDs, FilterLibraryIDs: filterLibraries,
		})
		if queryErr != nil {
			return nil, nil, nil, queryErr
		}
		novels = result.Comics
	}
	sessions, err := store.GetReadingSessionRecords(getUserID(c), libraryIDs, filterLibraries)
	if err != nil {
		return nil, nil, nil, err
	}
	return works, novels, sessions, nil
}

// GET /api/stats/files — 文件统计
func (h *StatsHandler) GetFileStats(c *gin.Context) {
	stats, err := store.GetFileStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get file stats"})
		return
	}
	c.JSON(http.StatusOK, stats)
}

// GET /api/stats/folder-tree — 文件夹树形统计
func (h *StatsHandler) GetFolderTreeStats(c *gin.Context) {
	tree, err := store.GetFolderTreeStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get folder tree stats"})
		return
	}
	c.JSON(http.StatusOK, tree)
}
