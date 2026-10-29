package apitest

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"openscum/internal/config"
)

type fileDTO struct {
	ID          uuid.UUID `json:"id"`
	Kind        string    `json:"kind"`
	Name        string    `json:"name"`
	Size        int64     `json:"size"`
	ContentType string    `json:"content_type"`
}

var (
	pngData = append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 64)...)
	pdfData = []byte("%PDF-1.7\n1 0 obj\n")
)

func (s *workSetup) upload(t *testing.T, c *Client, owner, name string, data []byte, status int, code ...string) fileDTO {
	t.Helper()
	var f fileDTO
	res := c.Upload("/api/"+owner+"/files", name, data).Expect(t, status, code...)
	if status == http.StatusCreated {
		res.JSON(t, &f)
	}
	return f
}

// WithS3 points the files at the S3 of the tests, or skips the test without it.
func WithS3(t *testing.T) Option {
	t.Helper()
	endpoint := os.Getenv("TEST_S3_ENDPOINT")
	if endpoint == "" {
		t.Skip("sin TEST_S3_ENDPOINT")
	}
	cfg := config.FilesConfig{
		Endpoint: endpoint, Bucket: "kairos-apitest",
		AccessKey: os.Getenv("TEST_S3_ACCESS_KEY"), SecretKey: os.Getenv("TEST_S3_SECRET_KEY"),
	}
	client, err := minio.New(cfg.Endpoint, &minio.Options{Creds: credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, "")})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if ok, err := client.BucketExists(ctx, cfg.Bucket); err != nil {
		t.Fatal(err)
	} else if !ok {
		if err := client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	return func(c *config.Config) { c.Files = cfg }
}

// Uploading and serving files (RF-14 to RF-18, RF-23, RF-24).
func TestUploadAndServeFiles(t *testing.T) {
	s := newWorkSetup(t)
	wi := s.item(t, s.member, s.workProject(t), map[string]any{})
	owner := "work-items/" + wi.ID.String()

	img := s.upload(t, s.member, owner, "captura.png", pngData, http.StatusCreated)
	if img.Kind != "image" || img.ContentType != "image/png" || img.Size != int64(len(pngData)) {
		t.Fatalf("image = %+v", img)
	}
	res := s.member.Get("/api/files/"+img.ID.String()).Expect(t, http.StatusOK)
	if !bytes.Equal(res.Body, pngData) {
		t.Fatalf("served %d bytes", len(res.Body))
	}
	for k, want := range map[string]string{
		"Content-Type": "image/png", "Content-Disposition": "inline; filename*=UTF-8''captura.png",
		"X-Content-Type-Options": "nosniff", "Content-Security-Policy": "default-src 'none'; sandbox",
	} {
		if got := res.Header.Get(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}

	// A PNG named .pdf is an image; a real PDF downloads with its name.
	if f := s.upload(t, s.member, owner, "disfrazado.pdf", pngData, http.StatusCreated); f.Kind != "image" {
		t.Errorf("disguised = %+v", f)
	}
	pdf := s.upload(t, s.member, owner, "Contrato IdP.pdf", pdfData, http.StatusCreated)
	res = s.member.Get("/api/files/"+pdf.ID.String()).Expect(t, http.StatusOK)
	if got := res.Header.Get("Content-Disposition"); got != "attachment; filename*=UTF-8''Contrato%20IdP.pdf" {
		t.Errorf("disposition = %q", got)
	}

	// Types and sizes (RF-16, RF-17).
	s.upload(t, s.member, owner, "dibujo.svg", []byte(`<svg onload="alert(1)"/>`), http.StatusUnsupportedMediaType, "file_type_not_allowed")
	s.upload(t, s.member, owner, "x.html", []byte("<html><script>alert(1)</script>"), http.StatusUnsupportedMediaType, "file_type_not_allowed")
	s.upload(t, s.member, owner, "max.txt", bytes.Repeat([]byte("a"), 10<<20), http.StatusCreated)
	s.upload(t, s.member, owner, "big.txt", bytes.Repeat([]byte("a"), 10<<20+1), http.StatusRequestEntityTooLarge, "file_too_large")

	// Reading follows the page: every project is readable with work:read (spec 004).
	// Uploading needs the right to change the work item.
	s.viewer.Get("/api/files/"+img.ID.String()).Expect(t, http.StatusOK)
	s.outsider.Get("/api/files/"+img.ID.String()).Expect(t, http.StatusOK)
	s.upload(t, s.viewer, owner, "a.png", pngData, http.StatusForbidden)
	s.upload(t, s.outsider, owner, "a.png", pngData, http.StatusForbidden)
	s.member.Get("/api/files/"+uuid.NewString()).Expect(t, http.StatusNotFound)
	if res := s.env.Client().Get("/api/files/" + img.ID.String()); res.Status != http.StatusUnauthorized {
		t.Errorf("without a session = %d", res.Status)
	}
}

// Milestones and projects take files from whoever manages the project; a
// page has no limit on the number of files (RF-43, RF-55).
func TestFilesOfMilestonesAndProjects(t *testing.T) {
	s := newWorkSetup(t)
	project := s.workProject(t)
	m := s.milestone(t, project, map[string]any{"name": "Beta", "due_date": "2026-12-01"})
	s.upload(t, s.lead, "milestones/"+m.ID.String(), "a.png", pngData, http.StatusCreated)
	s.upload(t, s.member, "milestones/"+m.ID.String(), "a.png", pngData, http.StatusForbidden)
	s.upload(t, s.member, "projects/"+project.String(), "a.png", pngData, http.StatusForbidden)
	for i := 0; i < 25; i++ {
		s.upload(t, s.lead, "projects/"+project.String(), "f.png", pngData, http.StatusCreated)
	}
	s.upload(t, s.lead, "projects/"+uuid.NewString(), "a.png", pngData, http.StatusNotFound)
}

// With S3 configured the files go there and are served the same (RF-21).
func TestFilesInS3(t *testing.T) {
	env := New(t, WithS3(t))
	s := newWorkSetupOn(t, env)
	wi := s.item(t, s.member, s.workProject(t), map[string]any{})
	f := s.upload(t, s.member, "work-items/"+wi.ID.String(), "notas.txt", []byte("hola"), http.StatusCreated)
	var storage string
	if err := env.Store.Pool.QueryRow(context.Background(), "SELECT storage FROM page_files WHERE id = $1", f.ID).Scan(&storage); err != nil || storage != "s3" {
		t.Fatalf("storage = %q, %v", storage, err)
	}
	res := s.member.Get("/api/files/"+f.ID.String()).Expect(t, http.StatusOK)
	if string(res.Body) != "hola" || !strings.HasPrefix(res.Header.Get("Content-Type"), "text/plain") {
		t.Fatalf("served %q %s", res.Body, res.Header.Get("Content-Type"))
	}
}
