package archive

import (
	"bytes"
	"compress/zlib"
	"encoding/ascii85"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	"image/png"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	rscpdf "rsc.io/pdf"
)

const (
	maxPDFCoverPixels      = 120_000_000
	maxPDFCoverStreamBytes = 512 << 20
	maxPDFResourceDepth    = 4
)

type pdfImageCandidate struct {
	value  rscpdf.Value
	width  int
	height int
	area   int64
}

// ExtractPDFPagePrimaryImage extracts the largest image XObject from a PDF page.
// It is intended as a renderer-free fallback for image-based manga PDFs where a
// page is usually backed by a single JPEG image stream.
func ExtractPDFPagePrimaryImage(filePath string, pageIndex int) (data []byte, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			data = nil
			err = fmt.Errorf("parse PDF image stream: %v", recovered)
		}
	}()

	if pageIndex < 0 {
		return nil, fmt.Errorf("invalid PDF page index: %d", pageIndex)
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open PDF: %w", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat PDF: %w", err)
	}

	reader, err := rscpdf.NewReader(file, info.Size())
	if err != nil {
		return nil, fmt.Errorf("parse PDF: %w", err)
	}
	if pageIndex >= reader.NumPage() {
		return nil, fmt.Errorf("PDF page %d out of range", pageIndex+1)
	}

	page := reader.Page(pageIndex + 1)
	if page.V.IsNull() {
		return nil, fmt.Errorf("PDF page %d not found", pageIndex+1)
	}

	var candidates []pdfImageCandidate
	collectPDFImageCandidates(page.Resources(), 0, make(map[string]struct{}), &candidates)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("PDF page %d has no image XObject", pageIndex+1)
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].area > candidates[j].area
	})

	var firstErr error
	for _, candidate := range candidates {
		imageData, decodeErr := decodePDFImageCandidate(file, candidate)
		if decodeErr == nil && len(imageData) > 0 {
			return imageData, nil
		}
		if firstErr == nil {
			firstErr = decodeErr
		}
	}
	if firstErr == nil {
		firstErr = fmt.Errorf("no supported PDF image stream")
	}
	return nil, fmt.Errorf("extract PDF page %d image: %w", pageIndex+1, firstErr)
}

func collectPDFImageCandidates(resources rscpdf.Value, depth int, seen map[string]struct{}, out *[]pdfImageCandidate) {
	if resources.IsNull() || depth > maxPDFResourceDepth {
		return
	}

	xObjects := resources.Key("XObject")
	for _, key := range xObjects.Keys() {
		object := xObjects.Key(key)
		if object.IsNull() {
			continue
		}

		identity := object.String()
		if _, exists := seen[identity]; exists {
			continue
		}
		seen[identity] = struct{}{}

		switch object.Key("Subtype").Name() {
		case "Image":
			width := int(object.Key("Width").Int64())
			height := int(object.Key("Height").Int64())
			if width <= 0 || height <= 0 {
				continue
			}
			area := int64(width) * int64(height)
			if area <= 0 || area > maxPDFCoverPixels {
				continue
			}
			*out = append(*out, pdfImageCandidate{
				value:  object,
				width:  width,
				height: height,
				area:   area,
			})
		case "Form":
			collectPDFImageCandidates(object.Key("Resources"), depth+1, seen, out)
		}
	}
}

func decodePDFImageCandidate(file *os.File, candidate pdfImageCandidate) ([]byte, error) {
	raw, err := readRawPDFStream(file, candidate.value)
	if err != nil {
		return nil, err
	}

	filters := pdfValueNames(candidate.value.Key("Filter"))
	decoded := raw
	encodedImage := false

	for index, filter := range filters {
		switch filter {
		case "ASCII85Decode", "A85":
			decoded, err = decodeASCII85(decoded)
		case "ASCIIHexDecode", "AHx":
			decoded, err = decodeASCIIHex(decoded)
		case "FlateDecode", "Fl":
			decoded, err = decodeFlate(decoded)
			if err == nil {
				params := pdfDecodeParams(candidate.value.Key("DecodeParms"), index)
				components := pdfImageComponents(candidate.value.Key("ColorSpace"))
				bits := int(candidate.value.Key("BitsPerComponent").Int64())
				if bits == 0 {
					bits = 8
				}
				decoded, err = applyPDFPredictor(decoded, params, candidate.width, candidate.height, components, bits)
			}
		case "RunLengthDecode", "RL":
			decoded, err = decodeRunLength(decoded)
		case "DCTDecode", "DCT":
			encodedImage = true
		case "JPXDecode":
			return nil, fmt.Errorf("JPEG 2000 PDF image streams are not supported by the native fallback")
		default:
			return nil, fmt.Errorf("unsupported PDF image filter %q", filter)
		}
		if err != nil {
			return nil, fmt.Errorf("decode PDF filter %q: %w", filter, err)
		}
	}

	if encodedImage || len(filters) == 0 {
		if _, _, decodeErr := image.DecodeConfig(bytes.NewReader(decoded)); decodeErr == nil {
			return decoded, nil
		}
		if encodedImage {
			return nil, fmt.Errorf("decoded PDF image stream is not a supported image")
		}
	}

	return encodePDFRasterImage(decoded, candidate)
}

func readRawPDFStream(file *os.File, value rscpdf.Value) ([]byte, error) {
	length := value.Key("Length").Int64()
	if length <= 0 || length > maxPDFCoverStreamBytes {
		return nil, fmt.Errorf("invalid PDF image stream length: %d", length)
	}

	offset, err := pdfStreamOffset(value.String())
	if err != nil {
		return nil, err
	}

	data := make([]byte, int(length))
	if _, err := io.ReadFull(io.NewSectionReader(file, offset, length), data); err != nil {
		return nil, fmt.Errorf("read PDF image stream: %w", err)
	}
	return data, nil
}

func pdfStreamOffset(description string) (int64, error) {
	separator := strings.LastIndex(description, "@")
	if separator < 0 || separator == len(description)-1 {
		return 0, fmt.Errorf("PDF stream offset unavailable")
	}
	offset, err := strconv.ParseInt(strings.TrimSpace(description[separator+1:]), 10, 64)
	if err != nil || offset < 0 {
		return 0, fmt.Errorf("invalid PDF stream offset")
	}
	return offset, nil
}

func pdfValueNames(value rscpdf.Value) []string {
	switch value.Kind() {
	case rscpdf.Null:
		return nil
	case rscpdf.Name:
		return []string{value.Name()}
	case rscpdf.Array:
		names := make([]string, 0, value.Len())
		for index := 0; index < value.Len(); index++ {
			if name := value.Index(index).Name(); name != "" {
				names = append(names, name)
			}
		}
		return names
	default:
		return nil
	}
}

func pdfDecodeParams(value rscpdf.Value, filterIndex int) rscpdf.Value {
	if value.Kind() == rscpdf.Array {
		return value.Index(filterIndex)
	}
	return value
}

func pdfImageComponents(colorSpace rscpdf.Value) int {
	name := colorSpace.Name()
	if colorSpace.Kind() == rscpdf.Array {
		name = colorSpace.Index(0).Name()
		if name == "ICCBased" {
			return int(colorSpace.Index(1).Key("N").Int64())
		}
	}

	switch name {
	case "DeviceGray", "CalGray":
		return 1
	case "DeviceRGB", "CalRGB":
		return 3
	case "DeviceCMYK":
		return 4
	default:
		return 0
	}
}

func decodeASCII85(data []byte) ([]byte, error) {
	trimmed := bytes.TrimSpace(data)
	trimmed = bytes.TrimPrefix(trimmed, []byte("<~"))
	trimmed = bytes.TrimSuffix(trimmed, []byte("~>"))
	decoded, err := io.ReadAll(io.LimitReader(ascii85.NewDecoder(bytes.NewReader(trimmed)), maxPDFCoverStreamBytes+1))
	if err != nil {
		return nil, err
	}
	if len(decoded) > maxPDFCoverStreamBytes {
		return nil, fmt.Errorf("decoded PDF image stream is too large")
	}
	return decoded, nil
}

func decodeASCIIHex(data []byte) ([]byte, error) {
	cleaned := make([]byte, 0, len(data))
	for _, value := range data {
		if value == '>' {
			break
		}
		switch value {
		case ' ', '\t', '\r', '\n', '\f', 0:
			continue
		default:
			cleaned = append(cleaned, value)
		}
	}
	if len(cleaned)%2 != 0 {
		cleaned = append(cleaned, '0')
	}
	decoded := make([]byte, hex.DecodedLen(len(cleaned)))
	if _, err := hex.Decode(decoded, cleaned); err != nil {
		return nil, err
	}
	return decoded, nil
}

func decodeFlate(data []byte) ([]byte, error) {
	reader, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	decoded, err := io.ReadAll(io.LimitReader(reader, maxPDFCoverStreamBytes+1))
	if err != nil {
		return nil, err
	}
	if len(decoded) > maxPDFCoverStreamBytes {
		return nil, fmt.Errorf("decoded PDF image stream is too large")
	}
	return decoded, nil
}

func decodeRunLength(data []byte) ([]byte, error) {
	decoded := make([]byte, 0, len(data))
	for index := 0; index < len(data); {
		length := int(data[index])
		index++
		switch {
		case length == 128:
			return decoded, nil
		case length <= 127:
			count := length + 1
			if index+count > len(data) {
				return nil, fmt.Errorf("truncated PDF run-length stream")
			}
			decoded = append(decoded, data[index:index+count]...)
			index += count
		case length >= 129:
			if index >= len(data) {
				return nil, fmt.Errorf("truncated PDF run-length repeat")
			}
			count := 257 - length
			for repeat := 0; repeat < count; repeat++ {
				decoded = append(decoded, data[index])
			}
			index++
		}
		if len(decoded) > maxPDFCoverStreamBytes {
			return nil, fmt.Errorf("decoded PDF image stream is too large")
		}
	}
	return decoded, nil
}

func applyPDFPredictor(data []byte, params rscpdf.Value, width, height, components, bits int) ([]byte, error) {
	predictor := int(params.Key("Predictor").Int64())
	if predictor <= 1 || params.IsNull() {
		return data, nil
	}
	if components <= 0 || bits <= 0 || width <= 0 || height <= 0 {
		return nil, fmt.Errorf("invalid PDF predictor dimensions")
	}

	columns := int(params.Key("Columns").Int64())
	if columns <= 0 {
		columns = width
	}
	colors := int(params.Key("Colors").Int64())
	if colors <= 0 {
		colors = components
	}
	predictorBits := int(params.Key("BitsPerComponent").Int64())
	if predictorBits <= 0 {
		predictorBits = bits
	}
	if predictorBits != 8 {
		return nil, fmt.Errorf("unsupported PDF predictor bit depth: %d", predictorBits)
	}

	rowLength := columns * colors
	bytesPerPixel := colors
	if rowLength <= 0 || bytesPerPixel <= 0 {
		return nil, fmt.Errorf("invalid PDF predictor row size")
	}

	switch predictor {
	case 2:
		if len(data) < rowLength*height {
			return nil, fmt.Errorf("truncated TIFF predictor data")
		}
		output := append([]byte(nil), data[:rowLength*height]...)
		for row := 0; row < height; row++ {
			start := row * rowLength
			for column := bytesPerPixel; column < rowLength; column++ {
				output[start+column] += output[start+column-bytesPerPixel]
			}
		}
		return output, nil
	case 10, 11, 12, 13, 14, 15:
		return decodePNGPredictor(data, predictor, rowLength, bytesPerPixel, height)
	default:
		return nil, fmt.Errorf("unsupported PDF predictor: %d", predictor)
	}
}

func decodePNGPredictor(data []byte, predictor, rowLength, bytesPerPixel, rows int) ([]byte, error) {
	hasFilterByte := len(data) >= rows*(rowLength+1)
	if !hasFilterByte && len(data) < rows*rowLength {
		return nil, fmt.Errorf("truncated PNG predictor data")
	}

	output := make([]byte, rows*rowLength)
	previous := make([]byte, rowLength)
	offset := 0
	for row := 0; row < rows; row++ {
		filter := byte(predictor - 10)
		if hasFilterByte {
			filter = data[offset]
			offset++
		}
		if offset+rowLength > len(data) {
			return nil, fmt.Errorf("truncated PNG predictor row")
		}
		current := output[row*rowLength : (row+1)*rowLength]
		copy(current, data[offset:offset+rowLength])
		offset += rowLength

		for column := 0; column < rowLength; column++ {
			left := byte(0)
			if column >= bytesPerPixel {
				left = current[column-bytesPerPixel]
			}
			up := previous[column]
			upperLeft := byte(0)
			if column >= bytesPerPixel {
				upperLeft = previous[column-bytesPerPixel]
			}

			switch filter {
			case 0:
			case 1:
				current[column] += left
			case 2:
				current[column] += up
			case 3:
				current[column] += byte((int(left) + int(up)) / 2)
			case 4:
				current[column] += paethPredictor(left, up, upperLeft)
			default:
				return nil, fmt.Errorf("unsupported PNG predictor filter: %d", filter)
			}
		}
		copy(previous, current)
	}
	return output, nil
}

func paethPredictor(left, up, upperLeft byte) byte {
	prediction := int(left) + int(up) - int(upperLeft)
	leftDistance := absInt(prediction - int(left))
	upDistance := absInt(prediction - int(up))
	upperLeftDistance := absInt(prediction - int(upperLeft))
	if leftDistance <= upDistance && leftDistance <= upperLeftDistance {
		return left
	}
	if upDistance <= upperLeftDistance {
		return up
	}
	return upperLeft
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func encodePDFRasterImage(data []byte, candidate pdfImageCandidate) ([]byte, error) {
	components := pdfImageComponents(candidate.value.Key("ColorSpace"))
	bits := int(candidate.value.Key("BitsPerComponent").Int64())
	if bits == 0 {
		bits = 8
	}
	if bits != 8 {
		return nil, fmt.Errorf("unsupported PDF image bit depth: %d", bits)
	}
	if components != 1 && components != 3 && components != 4 {
		return nil, fmt.Errorf("unsupported PDF image color space")
	}

	required := candidate.width * candidate.height * components
	if required <= 0 || len(data) < required {
		return nil, fmt.Errorf("truncated PDF raster image: got %d bytes, need %d", len(data), required)
	}

	output := image.NewRGBA(image.Rect(0, 0, candidate.width, candidate.height))
	position := 0
	for y := 0; y < candidate.height; y++ {
		for x := 0; x < candidate.width; x++ {
			switch components {
			case 1:
				gray := data[position]
				position++
				output.SetRGBA(x, y, color.RGBA{R: gray, G: gray, B: gray, A: 255})
			case 3:
				output.SetRGBA(x, y, color.RGBA{
					R: data[position],
					G: data[position+1],
					B: data[position+2],
					A: 255,
				})
				position += 3
			case 4:
				red, green, blue := color.CMYKToRGB(data[position], data[position+1], data[position+2], data[position+3])
				output.SetRGBA(x, y, color.RGBA{R: red, G: green, B: blue, A: 255})
				position += 4
			}
		}
	}

	var buffer bytes.Buffer
	if err := png.Encode(&buffer, output); err != nil {
		return nil, fmt.Errorf("encode extracted PDF image: %w", err)
	}
	return buffer.Bytes(), nil
}
