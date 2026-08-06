//go:build windows

package handler

import (
	"fmt"
	"syscall"
	"unsafe"

	"github.com/nowen-reader/nowen-reader/internal/config"
)

func checkDiskSpacePlatform() []DiagnosticItem {
	var items []DiagnosticItem

	dataDir := config.DataDir()
	if dataDir == "" {
		return items
	}

	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	getDiskFreeSpaceEx := kernel32.NewProc("GetDiskFreeSpaceExW")

	var freeBytesAvailable, totalBytes, totalFreeBytes int64
	dirPtr, err := syscall.UTF16PtrFromString(dataDir)
	if err != nil {
		items = append(items, DiagnosticItem{
			ID:      "disk-space",
			Name:    "磁盘空间",
			Status:  "warning",
			Message: "无法获取磁盘空间信息",
		})
		return items
	}

	ret, _, _ := getDiskFreeSpaceEx.Call(
		uintptr(unsafe.Pointer(dirPtr)),
		uintptr(unsafe.Pointer(&freeBytesAvailable)),
		uintptr(unsafe.Pointer(&totalBytes)),
		uintptr(unsafe.Pointer(&totalFreeBytes)),
	)
	if ret == 0 {
		items = append(items, DiagnosticItem{
			ID:      "disk-space",
			Name:    "磁盘空间",
			Status:  "warning",
			Message: "无法获取磁盘空间信息",
		})
		return items
	}

	usedBytes := totalBytes - totalFreeBytes
	usedPercent := 0
	if totalBytes > 0 {
		usedPercent = int(usedBytes * 100 / totalBytes)
	}

	status := "ok"
	message := fmt.Sprintf("已用 %d%% (%s / %s)",
		usedPercent, formatBytes(usedBytes), formatBytes(totalBytes))

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
		Detail:  fmt.Sprintf("可用: %s", formatBytes(freeBytesAvailable)),
	})

	return items
}
