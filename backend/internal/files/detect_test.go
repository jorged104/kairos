package files

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

func zipWith(t *testing.T, names ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, n := range names {
		f, err := w.Create(n)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = f.Write([]byte("<x/>"))
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

var (
	png  = append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 32)...)
	webp = append([]byte("RIFF\x00\x00\x00\x00WEBPVP8 "), make([]byte, 32)...)
	ole2 = append(append([]byte{}, oleMagic...), make([]byte, 64)...)
)

// The type comes from the content, not the extension (RF-14, RF-15, RF-18).
func TestDetectAdmits(t *testing.T) {
	docx := zipWith(t, "[Content_Types].xml", "word/document.xml")
	cases := []struct {
		name string
		data []byte
		kind string
		typ  string
	}{
		{"captura.png", png, "image", "image/png"},
		{"captura.pdf", png, "image", "image/png"}, // a PNG named .pdf is still an image
		{"foto.jpg", []byte("\xFF\xD8\xFF\xE0\x00\x10JFIF\x00"), "image", "image/jpeg"},
		{"anim.gif", []byte("GIF89a\x01\x00\x01\x00"), "image", "image/gif"},
		{"x.webp", webp, "image", "image/webp"},
		{"contrato.pdf", []byte("%PDF-1.7\n..."), "file", "application/pdf"},
		{"informe.docx", docx, "file", ooxml[".docx"]},
		{"datos.xlsx", zipWith(t, "[Content_Types].xml", "xl/workbook.xml"), "file", ooxml[".xlsx"]},
		{"viejo.doc", ole2, "file", "application/msword"},
		{"viejo.xls", ole2, "file", "application/vnd.ms-excel"},
		{"notas.txt", []byte("Hola, ñandú\n"), "file", "text/plain; charset=utf-8"},
		{"datos.csv", []byte("a,b\n1,2\n"), "file", "text/csv; charset=utf-8"},
		{"todo.zip", docx, "file", "application/zip"},
	}
	for _, c := range cases {
		kind, typ, err := detect(c.name, c.data)
		if err != nil || kind != c.kind || typ != c.typ {
			t.Errorf("%s: %s %s %v; want %s %s", c.name, kind, typ, err, c.kind, c.typ)
		}
	}
}

// SVG, HTML, executables and disguised containers are not admitted (RF-17).
func TestDetectRejects(t *testing.T) {
	cases := map[string][]byte{
		"dibujo.svg":     []byte(`<svg xmlns="http://www.w3.org/2000/svg" onload="alert(1)"></svg>`),
		"xml.svg":        []byte(`<?xml version="1.0"?><svg/>`),
		"pagina.txt":     []byte("<!DOCTYPE html><html><script>alert(1)</script>"),
		"pagina.html":    []byte("<html><body>x</body></html>"),
		"setup.exe":      append([]byte("MZ\x90\x00"), make([]byte, 64)...),
		"setup.txt":      append([]byte("MZ\x90\x00"), make([]byte, 64)...),
		"binario.pdf":    append([]byte("\x7fELF"), make([]byte, 64)...),
		"falso.docx":     zipWith(t, "word/document.xml"), // a zip without [Content_Types].xml
		"app.jar":        zipWith(t, "META-INF/MANIFEST.MF"),
		"instalador.msi": ole2,
		"datos.json":     []byte(`{"a":1}`),
		"latin1.txt":     []byte("caf\xe9"),
	}
	for name, data := range cases {
		if _, _, err := detect(name, data); err != errFileTypeNotAllowed {
			t.Errorf("%s: err = %v, want file_type_not_allowed", name, err)
		}
	}
}

func TestCleanName(t *testing.T) {
	cases := map[string]string{
		"informe.pdf":            "informe.pdf",
		`C:\Users\ana\foto.png`:  "foto.png",
		"../../etc/passwd":       "passwd",
		"  a\x00b\nc.txt ":       "abc.txt",
		"":                       "archivo",
		strings.Repeat("a", 300): strings.Repeat("a", 255),
	}
	for in, want := range cases {
		if got := cleanName(in); got != want {
			t.Errorf("cleanName(%q) = %q, want %q", in, got, want)
		}
	}
	if got := cleanName(strings.Repeat("ñ", 300) + ".pdf"); len([]rune(got)) != 255 || !strings.HasSuffix(got, ".pdf") {
		t.Errorf("long name = %q", got)
	}
}
