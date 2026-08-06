//go:build !windows

package handler

import (
	"fmt"
	"syscall"

	"github.com/nowen-reader/nowen-reader/internal/config"
)

func checkDiskSpacePlatform() []DiagnosticItem {
	var items []DiagnosticItem

	dataDir := config.DataDir()
	if dataDir == "" {
		return items
	}

	var stat syscall.Statfs_t
	if err := syscall.Statfs(dataDir, &stat); err != nil {
		items = append(items, DiagnosticItem{
			ID:      "disk-space",
			Name:    "磁盘空间",
			Status:  "warning",
			Message: "无法获取磁盘空间信息",
		})
		return items
	}

	totalBytes := stat.Blocks * uint64(stat.Bsize)
	freeBytes := stat.Bavail * uint64(stat.Bsize)
	usedBytes := totalBytes - freeBytes
	usedPercent := 0
	if totalBytes > 0 {
		usedPercent = int(usedBytes * 100 / totalBytes)
	}

	status := "ok"
	message := fmt.Sprintf("已用 %d%% (%s / %s)",
		usedPercent, formatBytes(int64(usedBytes)), formatBytes(int64(totalBytes)))

	if usedPercent > 90 {
		status = "error"
		message += " — 磁盘空间严重不足"
	} else if usedPercent > 80 {
		status = "warning"
		message += " — 磁盘空间紧张"
	}

	items = append(items, DiagnosticItem{
		ID:      "disk-space",
		Name:    "磁盘空间",
		Status:  status,
		Message: message,
		Detail:  fmt.Sprintf("可用: %s", formatBytes(int64(freeBytes))),
	})

	return items
}
