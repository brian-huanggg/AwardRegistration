package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
)

const (
	// DialTimeout bounds connection setup to MinIO (Backend.md §4.4, D50).
	DialTimeout = 2 * time.Second
	// CallTimeout bounds one whole MinIO call, including a streamed Get.
	CallTimeout = 30 * time.Second
)

// ErrObjectNotFound is returned by Get when the key does not exist. It is a
// consistency problem (a row points at a missing object), not an outage.
var ErrObjectNotFound = errors.New("storage: object not found")

// SlideStore is the MinIO port. Failures other than ErrObjectNotFound are
// *web.Error with code STORAGE_UNAVAILABLE (503).
type SlideStore interface {
	// Put writes size bytes from r under key. size must be exact (>= 0).
	Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error
	// Get opens key; the caller must Close the reader. size is the object length.
	Get(ctx context.Context, key string) (rc io.ReadCloser, size int64, err error)
	// Delete removes key. Deleting a missing key succeeds.
	Delete(ctx context.Context, key string) error
}

// MinIOConfig configures NewMinIO.
type MinIOConfig struct {
	Endpoint  string // host:port
	AccessKey string
	SecretKey string
	Bucket    string
	Secure    bool
}

type minioStore struct {
	c      *minio.Client
	bucket string
}

// NewMinIO builds the client: 2 s dial timeout, and MaxRetries: 1 — minio-go
// treats 0 as "default of 10", so 1 is the only way to get a single attempt
// (Backend.md §4.4, A3c). It makes no network call.
func NewMinIO(cfg MinIOConfig) (SlideStore, error) {
	tr := http.DefaultTransport.(*http.Transport).Clone() //nolint:forcetypeassert // stdlib guarantee
	tr.DialContext = (&net.Dialer{Timeout: DialTimeout, KeepAlive: 30 * time.Second}).DialContext
	c, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:      credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure:     cfg.Secure,
		Transport:  tr,
		MaxRetries: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("storage: minio client: %w", err)
	}
	return &minioStore{c: c, bucket: cfg.Bucket}, nil
}

func unavailable(op string, err error) error {
	e := web.NewError(web.CodeStorageUnavailable, "object storage is unavailable")
	e.Cause = fmt.Errorf("minio %s: %w", op, err)
	return e
}

func isNotFound(err error) bool {
	return minio.ToErrorResponse(err).Code == "NoSuchKey"
}

func (s *minioStore) Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error {
	ctx, cancel := context.WithTimeout(ctx, CallTimeout)
	defer cancel()
	_, err := s.c.PutObject(ctx, s.bucket, key, r, size, minio.PutObjectOptions{
		ContentType: contentType,
		// Single PUT for slide-sized objects; no multipart retries to reason about.
		DisableMultipart: true,
	})
	if err != nil {
		return unavailable("put", err)
	}
	return nil
}

func (s *minioStore) Get(ctx context.Context, key string) (io.ReadCloser, int64, error) {
	ctx, cancel := context.WithTimeout(ctx, CallTimeout)
	obj, err := s.c.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		cancel()
		return nil, 0, unavailable("get", err)
	}
	// GetObject is lazy; Stat performs the request, so a down server or a
	// missing key surfaces here, before the caller writes a first byte.
	st, err := obj.Stat()
	if err != nil {
		_ = obj.Close()
		cancel()
		if isNotFound(err) {
			return nil, 0, ErrObjectNotFound
		}
		return nil, 0, unavailable("stat", err)
	}
	return &cancelCloser{ReadCloser: obj, cancel: cancel}, st.Size, nil
}

func (s *minioStore) Delete(ctx context.Context, key string) error {
	ctx, cancel := context.WithTimeout(ctx, CallTimeout)
	defer cancel()
	if err := s.c.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{}); err != nil {
		if isNotFound(err) {
			return nil
		}
		return unavailable("delete", err)
	}
	return nil
}

// cancelCloser releases the call context when the stream is closed.
type cancelCloser struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (c *cancelCloser) Close() error {
	err := c.ReadCloser.Close()
	c.cancel()
	return err
}
