package archive

import (
	"bufio"
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	"io"
	"os"
)

const (
	// 大型漫画 PDF 的第一页图片通常位于文件前部。限制扫描窗口，避免为了封面
	// 顺序读取数百 MB 甚至数 GB 的文件。
	maxRawPDFJPEGScanBytes = 96 << 20
	// 防止异常 PDF 中缺少 EOI 标记时无限积累单个候选流。
	maxEmbeddedJPEGBytes = 64 << 20
	// 过滤 PDF 内的小图标、缩略预览和色彩配置附件。
	minEmbeddedJPEGDimension  = 200
	preferredEmbeddedJPEGPixels = 200_000
)

// ExtractFirstEmbeddedJPEG 流式扫描 PDF 前部，提取第一个尺寸足够的 JPEG。
//
// 该路径专门用于页数多、文件体积大的图片型漫画 PDF：它不解析完整交叉引用表，
// 也不会把整个 PDF 读入内存。若只发现较小但有效的 JPEG，则返回扫描范围内面积
// 最大的候选；完全找不到时由调用方继续走标准 PDF 解析器和外部渲染器。
func ExtractFirstEmbeddedJPEG(filePath string) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open PDF for JPEG scan: %w", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat PDF for JPEG scan: %w", err)
	}

	scanBytes := info.Size()
	if scanBytes > maxRawPDFJPEGScanBytes {
		scanBytes = maxRawPDFJPEGScanBytes
	}
	if scanBytes <= 0 {
		return nil, fmt.Errorf("empty PDF")
	}

	reader := bufio.NewReaderSize(io.LimitReader(file, scanBytes), 256<<10)
	chunk := make([]byte, 256<<10)
	var candidate bytes.Buffer
	candidate.Grow(512 << 10)

	var (
		inJPEG       bool
		previous     byte
		havePrevious bool
		best         []byte
		bestArea     int64
	)

	for {
		n, readErr := reader.Read(chunk)
		for index := 0; index < n; index++ {
			current := chunk[index]

			if !inJPEG {
				if havePrevious && previous == 0xff && current == 0xd8 {
					candidate.Reset()
					candidate.WriteByte(0xff)
					candidate.WriteByte(0xd8)
					inJPEG = true
				}
			} else {
				if candidate.Len() >= maxEmbeddedJPEGBytes {
					candidate.Reset()
					inJPEG = false
				} else {
					candidate.WriteByte(current)
					if previous == 0xff && current == 0xd9 {
						jpegData := append([]byte(nil), candidate.Bytes()...)
						candidate.Reset()
						inJPEG = false

						config, format, decodeErr := image.DecodeConfig(bytes.NewReader(jpegData))
						if decodeErr == nil && format == "jpeg" &&
							config.Width >= minEmbeddedJPEGDimension &&
							config.Height >= minEmbeddedJPEGDimension {
							area := int64(config.Width) * int64(config.Height)
							if area >= preferredEmbeddedJPEGPixels {
								return jpegData, nil
							}
							if area > bestArea {
								best = jpegData
								bestArea = area
							}
						}
					}
				}
			}

			previous = current
			havePrevious = true
		}

		if readErr != nil {
			if readErr != io.EOF {
				return nil, fmt.Errorf("scan PDF JPEG data: %w", readErr)
			}
			break
		}
	}

	if len(best) > 0 {
		return best, nil
	}
	return nil, fmt.Errorf("no suitable embedded JPEG found in first %d bytes", scanBytes)
}
