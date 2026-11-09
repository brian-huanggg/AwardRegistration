//go:build integration

package storage_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	tcminio "github.com/testcontainers/testcontainers-go/modules/minio"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/storage"
)

func startMinIO(t *testing.T) (storage.MinIOConfig, *tcminio.MinioContainer) {
	t.Helper()
	ctx := context.Background()
	ctr, err := tcminio.Run(ctx, "cgr.dev/chainguard/minio:latest") // same image as docker-compose.yml
	if err != nil {
		t.Fatalf("start minio: %v", err)
	}
	t.Cleanup(func() { _ = ctr.Terminate(context.WithoutCancel(ctx)) })
	ep, err := ctr.ConnectionString(ctx)
	if err != nil {
		t.Fatal(err)
	}
	cfg := storage.MinIOConfig{Endpoint: ep, AccessKey: ctr.Username, SecretKey: ctr.Password, Bucket: "slides"}
	admin, err := minio.New(ep, &minio.Options{Creds: credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, "")})
	if err != nil {
		t.Fatal(err)
	}
	if err := admin.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{}); err != nil {
		t.Fatal(err)
	}
	return cfg, ctr
}

func TestMinIORoundTrip(t *testing.T) {
	cfg, ctr := startMinIO(t)
	ctx := context.Background()
	s, err := storage.NewMinIO(cfg)
	if err != nil {
		t.Fatal(err)
	}

	body := "hello slides"
	if err := s.Put(ctx, "a/b.pdf", strings.NewReader(body), int64(len(body)), "application/pdf"); err != nil {
		t.Fatal(err)
	}
	rc, size, err := s.Get(ctx, "a/b.pdf")
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(rc)
	_ = rc.Close()
	if string(got) != body || size != int64(len(body)) {
		t.Fatalf("got %q size %d", got, size)
	}

	if _, _, err := s.Get(ctx, "missing"); !errors.Is(err, storage.ErrObjectNotFound) {
		t.Fatalf("missing key: %v", err)
	}
	if err := s.Delete(ctx, "a/b.pdf"); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx, "a/b.pdf"); err != nil {
		t.Fatalf("delete of missing key must succeed: %v", err)
	}
	if _, _, err := s.Get(ctx, "a/b.pdf"); !errors.Is(err, storage.ErrObjectNotFound) {
		t.Fatalf("after delete: %v", err)
	}

	// MinIO down → 503 STORAGE_UNAVAILABLE.
	if err := ctr.Terminate(ctx); err != nil {
		t.Fatal(err)
	}
	err = s.Put(ctx, "x", strings.NewReader("x"), 1, "application/pdf")
	var we *web.Error
	if !errors.As(err, &we) || we.Code != web.CodeStorageUnavailable {
		t.Fatalf("down: %v", err)
	}
}
