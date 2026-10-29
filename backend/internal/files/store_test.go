package files

import (
	"context"
	"io"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"openscum/internal/config"
	"openscum/internal/db"
	"openscum/internal/db/dbtest"
	"openscum/internal/db/sqlc"
)

// fixture is a database with a user and a project for the files to belong to.
type fixture struct {
	store         *db.Store
	user, project uuid.UUID
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()
	f := &fixture{store: db.NewStore(dbtest.New(t))}
	u, err := f.store.CreateUser(ctx, sqlc.CreateUserParams{Name: "Ana", Email: "ana@x.com", EmailNormalized: "ana@x.com", Status: "active"})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	f.user = u.ID
	var team uuid.UUID
	if err := f.store.Pool.QueryRow(ctx, "INSERT INTO teams (name, name_normalized) VALUES ('BI', 'bi') RETURNING id").Scan(&team); err != nil {
		t.Fatalf("team: %v", err)
	}
	if err := f.store.Pool.QueryRow(ctx, "INSERT INTO projects (team_id, owner_id, identifier, name) VALUES ($1, $2, 'KAI', 'Kairos') RETURNING id",
		team, f.user).Scan(&f.project); err != nil {
		t.Fatalf("project: %v", err)
	}
	return f
}

func (f *fixture) save(t *testing.T, s *Storage, content string) sqlc.PageFile {
	t.Helper()
	file, err := s.save(context.Background(), f.store.Queries, sqlc.CreatePageFileParams{
		ProjectID: f.project, OwnerKind: "project", OwnerID: f.project, Kind: "file", Name: "a.txt",
		ContentType: "text/plain; charset=utf-8", SizeBytes: int64(len(content)), CreatedBy: f.user,
	}, []byte(content))
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	return file
}

func (f *fixture) read(t *testing.T, s *Storage, file sqlc.PageFile) string {
	t.Helper()
	r, err := s.open(context.Background(), f.store.Queries, file)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer r.Close()
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return string(b)
}

// s3Config is the S3 of the tests (make up-s3, or the CI service). Without
// TEST_S3_ENDPOINT the S3 cases are skipped. The bucket is created if missing.
func s3Config(t *testing.T) config.FilesConfig {
	t.Helper()
	cfg := config.FilesConfig{
		Endpoint:  os.Getenv("TEST_S3_ENDPOINT"),
		Bucket:    "kairos-test",
		AccessKey: os.Getenv("TEST_S3_ACCESS_KEY"),
		SecretKey: os.Getenv("TEST_S3_SECRET_KEY"),
	}
	if cfg.Endpoint == "" {
		t.Skip("sin TEST_S3_ENDPOINT")
	}
	client, err := minio.New(cfg.Endpoint, &minio.Options{Creds: credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, "")})
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	ctx := context.Background()
	ok, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		t.Fatalf("bucket: %v", err)
	}
	if !ok {
		if err := client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{}); err != nil {
			t.Fatalf("make bucket: %v", err)
		}
	}
	return cfg
}

func storage(t *testing.T, cfg config.FilesConfig, mode string) *Storage {
	t.Helper()
	s, err := NewStorage(context.Background(), cfg)
	if err != nil || s.Mode() != mode {
		t.Fatalf("NewStorage = %v, %v; want %s", s, err, mode)
	}
	return s
}

// Saving, reading and copying in both storages (RF-21, RF-22, RF-53).
func TestStorageRoundTrip(t *testing.T) {
	cases := map[string]func(t *testing.T) config.FilesConfig{
		"db": func(*testing.T) config.FilesConfig { return config.FilesConfig{} },
		"s3": s3Config,
	}
	for mode, cfg := range cases {
		t.Run(mode, func(t *testing.T) {
			s := storage(t, cfg(t), mode)
			f := newFixture(t)
			file := f.save(t, s, "hola\n")
			if file.Storage != mode || (mode == "s3") != (file.S3Key != nil) {
				t.Fatalf("stored = %s %v, want %s", file.Storage, file.S3Key, mode)
			}
			if got := f.read(t, s, file); got != "hola\n" {
				t.Fatalf("content = %q", got)
			}
			c, err := s.copy(context.Background(), f.store.Queries, file,
				sqlc.CopyPageFileParams{ProjectID: f.project, OwnerID: uuid.New(), CreatedBy: f.user})
			if err != nil {
				t.Fatalf("copy: %v", err)
			}
			if c.ID == file.ID || c.Storage != mode || f.read(t, s, c) != "hola\n" {
				t.Fatalf("copy = %+v", c)
			}
			if mode == "s3" && *c.S3Key == *file.S3Key {
				t.Fatal("the copy shares the object of the original")
			}
		})
	}
}

// Files saved in the database stay readable once S3 is configured (RF-54).
func TestDatabaseFilesWithS3Configured(t *testing.T) {
	s3 := storage(t, s3Config(t), "s3")
	f := newFixture(t)
	old := f.save(t, storage(t, config.FilesConfig{}, "db"), "antes")
	if got := f.read(t, s3, old); got != "antes" {
		t.Fatalf("content = %q", got)
	}
}

func TestMissingBucketStopsTheAPI(t *testing.T) {
	cfg := s3Config(t)
	cfg.Bucket = "no-existe-" + uuid.NewString()[:8]
	if _, err := NewStorage(context.Background(), cfg); err == nil {
		t.Fatal("NewStorage accepted a missing bucket")
	}
}

// A file kept in S3 can't be read once S3 is no longer configured.
func TestS3FileWithoutS3(t *testing.T) {
	key := "pages/x"
	_, err := storage(t, config.FilesConfig{}, "db").open(context.Background(), nil, sqlc.PageFile{Storage: "s3", S3Key: &key})
	if err != errStorageUnavailable {
		t.Fatalf("open = %v, want storage_unavailable", err)
	}
}
