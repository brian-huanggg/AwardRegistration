// Command mkbucket creates the slides bucket in the local MinIO if it is missing (used by `make smoke`).
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

func run() error {
	c, err := minio.New(env("ESG_MINIO_ENDPOINT", "localhost:9000"), &minio.Options{
		Creds: credentials.NewStaticV4(env("ESG_MINIO_ACCESS_KEY", "minioadmin"), env("ESG_MINIO_SECRET_KEY", "minioadmin"), ""),
	})
	if err != nil {
		return err
	}
	bucket := env("ESG_MINIO_BUCKET", "esg-slides")
	ctx := context.Background()
	ok, err := c.BucketExists(ctx, bucket)
	if err != nil || ok {
		return err
	}
	return c.MakeBucket(ctx, bucket, minio.MakeBucketOptions{})
}
