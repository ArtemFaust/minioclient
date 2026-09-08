package utils

import (
	"context"
	"time"

	"github.com/minio/minio-go/v7"
)

func ConnectionHealthCheck(client *minio.Client) bool {
	context, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	_, e := client.ListBuckets(context)
	cancel()
	if e != nil {
		return false
	}
	return true
}
