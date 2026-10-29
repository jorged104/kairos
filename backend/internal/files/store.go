package files

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"openscum/internal/config"
	"openscum/internal/db/sqlc"
)

// Storage keeps the content of page files in S3 when it is configured and in
// the database otherwise (RF-21, RF-22). Each file records where it went, so
// files stay readable from the database after S3 is configured (RF-54).
type Storage struct {
	client *minio.Client
	bucket string
}

// NewStorage connects to S3 when cfg asks for it and checks that the bucket
// exists, so a wrong configuration stops the API at startup.
func NewStorage(ctx context.Context, cfg config.FilesConfig) (*Storage, error) {
	if !cfg.S3() {
		return &Storage{}, nil
	}
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("cliente S3: %w", err)
	}
	ok, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("comprobar el bucket %s: %w", cfg.Bucket, err)
	}
	if !ok {
		return nil, fmt.Errorf("el bucket %s no existe", cfg.Bucket)
	}
	return &Storage{client: client, bucket: cfg.Bucket}, nil
}

// Mode is where new files go: "s3" or "db".
func (s *Storage) Mode() string {
	if s.client != nil {
		return "s3"
	}
	return "db"
}

func objectKey(project uuid.UUID) string {
	return "pages/" + project.String() + "/" + uuid.NewString()
}

// save stores data and creates the file's row. With S3 the object is written
// first: if the transaction then fails, the object is left for the
// maintenance of a later spec, never a row without content.
func (s *Storage) save(ctx context.Context, q *sqlc.Queries, p sqlc.CreatePageFileParams, data []byte) (sqlc.PageFile, error) {
	p.Storage = s.Mode()
	if s.client != nil {
		key := objectKey(p.ProjectID)
		if _, err := s.client.PutObject(ctx, s.bucket, key, bytes.NewReader(data), int64(len(data)),
			minio.PutObjectOptions{ContentType: p.ContentType}); err != nil {
			return sqlc.PageFile{}, unavailable(fmt.Errorf("subir a S3: %w", err))
		}
		p.S3Key = &key
	}
	f, err := q.CreatePageFile(ctx, p)
	if err != nil {
		return sqlc.PageFile{}, fmt.Errorf("crear archivo: %w", err)
	}
	if s.client == nil {
		if err := q.CreatePageFileData(ctx, sqlc.CreatePageFileDataParams{FileID: f.ID, Data: data}); err != nil {
			return sqlc.PageFile{}, fmt.Errorf("guardar contenido: %w", err)
		}
	}
	return f, nil
}

// open returns the content of f from wherever it was stored.
func (s *Storage) open(ctx context.Context, q *sqlc.Queries, f sqlc.PageFile) (io.ReadCloser, error) {
	if f.Storage == "db" {
		data, err := q.GetPageFileData(ctx, f.ID)
		if err != nil {
			return nil, fmt.Errorf("leer contenido: %w", err)
		}
		return io.NopCloser(bytes.NewReader(data)), nil
	}
	if s.client == nil || f.S3Key == nil {
		return nil, errStorageUnavailable
	}
	obj, err := s.client.GetObject(ctx, s.bucket, *f.S3Key, minio.GetObjectOptions{})
	if err == nil {
		_, err = obj.Stat() // GetObject is lazy: this is where a missing object or a dead S3 shows
	}
	if err != nil {
		if obj != nil {
			_ = obj.Close()
		}
		return nil, unavailable(fmt.Errorf("leer de S3: %w", err))
	}
	return obj, nil
}

// copy duplicates f for another owner (RF-53), in the same storage as f.
func (s *Storage) copy(ctx context.Context, q *sqlc.Queries, f sqlc.PageFile, p sqlc.CopyPageFileParams) (sqlc.PageFile, error) {
	p.ID = f.ID
	if f.Storage == "s3" {
		if s.client == nil || f.S3Key == nil {
			return sqlc.PageFile{}, errStorageUnavailable
		}
		key := objectKey(p.ProjectID)
		if _, err := s.client.CopyObject(ctx,
			minio.CopyDestOptions{Bucket: s.bucket, Object: key},
			minio.CopySrcOptions{Bucket: s.bucket, Object: *f.S3Key}); err != nil {
			return sqlc.PageFile{}, unavailable(fmt.Errorf("copiar en S3: %w", err))
		}
		p.S3Key = &key
	}
	c, err := q.CopyPageFile(ctx, p)
	if err != nil {
		return sqlc.PageFile{}, fmt.Errorf("copiar archivo: %w", err)
	}
	if f.Storage == "db" {
		if err := q.CopyPageFileData(ctx, sqlc.CopyPageFileDataParams{NewID: c.ID, OldID: f.ID}); err != nil {
			return sqlc.PageFile{}, fmt.Errorf("copiar contenido: %w", err)
		}
	}
	return c, nil
}
