package files

import (
	"archive/zip"
	"bytes"
	"net/http"
	"path"
	"strings"
	"unicode"
	"unicode/utf8"
)

// MaxBytes is the size limit of one file (RF-16).
const MaxBytes = 10 << 20

var (
	oleMagic = []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}

	images = map[string]bool{"image/png": true, "image/jpeg": true, "image/gif": true, "image/webp": true}

	// Office formats by extension: OOXML is a zip with [Content_Types].xml,
	// the old formats are OLE compound files.
	ooxml = map[string]string{
		".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
	}
	ole = map[string]string{
		".doc": "application/msword",
		".xls": "application/vnd.ms-excel",
		".ppt": "application/vnd.ms-powerpoint",
	}
)

// detect decides the kind and type of a file by its content (RF-18), using
// the extension only to tell apart formats that share a container. Anything
// else, SVG, HTML and executables included, is not admitted (RF-17).
func detect(name string, data []byte) (kind, contentType string, err error) {
	ext := strings.ToLower(path.Ext(name))
	sniffed := http.DetectContentType(data)
	switch {
	case images[sniffed]:
		return "image", sniffed, nil
	case sniffed == "application/pdf":
		return "file", sniffed, nil
	case sniffed == "application/zip":
		if t, ok := ooxml[ext]; ok && hasContentTypes(data) {
			return "file", t, nil
		}
		if ext == ".zip" {
			return "file", "application/zip", nil
		}
	case bytes.HasPrefix(data, oleMagic):
		if t, ok := ole[ext]; ok {
			return "file", t, nil
		}
	case sniffed == "text/plain; charset=utf-8" && utf8.Valid(data):
		switch ext {
		case ".txt":
			return "file", "text/plain; charset=utf-8", nil
		case ".csv":
			return "file", "text/csv; charset=utf-8", nil
		}
	}
	return "", "", errFileTypeNotAllowed
}

func hasContentTypes(data []byte) bool {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return false
	}
	for _, f := range r.File {
		if f.Name == "[Content_Types].xml" {
			return true
		}
	}
	return false
}

// cleanName keeps the last path element of the name the browser sent, without
// control characters, at most 255 characters.
func cleanName(name string) string {
	name = path.Base(strings.ReplaceAll(name, "\\", "/"))
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, strings.TrimSpace(name))
	if name == "" || name == "." || name == "/" {
		name = "archivo"
	}
	if utf8.RuneCountInString(name) > 255 {
		ext := path.Ext(name)
		if utf8.RuneCountInString(ext) > 20 {
			ext = ""
		}
		runes := []rune(strings.TrimSuffix(name, ext))
		name = string(runes[:255-utf8.RuneCountInString(ext)]) + ext
	}
	return name
}
