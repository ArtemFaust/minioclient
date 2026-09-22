package utils

import (
	"context"
	"minioclient/global"
	"time"
)

func ConnectionHealthCheck(client *global.GlobalClient) bool {
	context, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	_, e := client.MinioClient.ListBuckets(context)
	cancel()
	if e != nil {
		return false
	}
	return true
}
