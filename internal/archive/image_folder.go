package archive

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nowen-reader/nowen-reader/internal/config"
)

// ============================================================
// 图片文件夹 Reader（将包含图片的文件夹视为一种归档格式）
// ============================================================

// imageFolderReader 实现 Reader 接口，用于读取图片文件夹漫画。
// 目录结构：漫画文件夹/001.jpg, 002.jpg, ...
type imageFolderReader struct {
	dirPath string  // 文件夹的绝对路径
	entries []Entry // 目录中的图片文件列表
}

// newImageFolderReader 创建一个图片文件夹 Reader。
func newImageFolderReader(dirPath string) (*imageFolderReader, error) {
	// 去除可能的尾部斜杠
	dirPath = strings.TrimRight(dirPath, "/\\")

	info, err := os.Stat(dirPath)
	if err != nil {
		return nil, fmt.Errorf("open image folder %s: %w", dirPath, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", dirPath)
	}

	r := &imageFolderReader{
		dirPath: dirPath,
		entries: make([]Entry, 0),
	}

	// 递归扫描目录中的所有图片文件
	err = filepath.WalkDir(dirPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // 跳过不可访问的文件
		}
		if d.IsDir() {
			// 跳过隐藏目录和 macOS 资源目录
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "__MACOSX" {
				return filepath.SkipDir
			}
			return nil
		}

		// 只收集图片文件
		if !config.IsImageFile(d.Name()) {
			return nil
		}

		// 跳过隐藏文件
		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}

		// 使用相对于 dirPath 的路径作为 entry name
		relPath, err := filepath.Rel(dirPath, path)
		if err != nil {
			return nil
		}
		// 统一使用正斜杠
		relPath = filepath.ToSlash(relPath)

		r.entries = append(r.entries, Entry{
			Name:        relPath,
			IsDirectory: false,
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan image folder %s: %w", dirPath, err)
	}

	return r, nil
}

func (r *imageFolderReader) ListEntries() []Entry {
	return r.entries
}

func (r *imageFolderReader) ExtractEntry(entryName string) ([]byte, error) {
	// entryName 是相对于 dirPath 的路径
	fullPath := filepath.Join(r.dirPath, filepath.FromSlash(entryName))

	// 安全检查：确保路径在 dirPath 内，防止路径遍历攻击
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return nil, fmt.Errorf("resolve path %s: %w", entryName, err)
	}
	absDir, err := filepath.Abs(r.dirPath)
	if err != nil {
		return nil, fmt.Errorf("resolve dir %s: %w", r.dirPath, err)
	}
	if !strings.HasPrefix(absPath, absDir) {
		return nil, fmt.Errorf("path traversal detected: %s", entryName)
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("read image %s: %w", entryName, err)
	}
	return data, nil
}

func (r *imageFolderReader) Close() {
	// 无需清理资源
}

// IsImageFolder 检查一个目录是否为图片文件夹漫画（包含至少一张图片文件）。
func IsImageFolder(dirPath string) bool {
	info, err := os.Stat(dirPath)
	if err != nil || !info.IsDir() {
		return false
	}

	// 检查目录中是否有图片文件（只需找到一个即可）
	hasImage := false
	filepath.WalkDir(dirPath, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if config.IsImageFile(d.Name()) && !strings.HasPrefix(d.Name(), ".") {
			hasImage = true
			return filepath.SkipAll // 找到一个就够了
		}
		return nil
	})
	return hasImage
}
