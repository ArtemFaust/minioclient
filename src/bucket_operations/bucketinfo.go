package bucketoperations

import (
	"context"
	"encoding/json"
	"errors"
	customoperations "minioclient/custom_operations"
	"minioclient/global"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/cors"
	"github.com/minio/minio-go/v7/pkg/lifecycle"
	"github.com/minio/minio-go/v7/pkg/notification"
)

type BucketInfo struct {
	Name                      *string
	Versioning                minio.BucketVersioningConfiguration
	Location                  string
	LifeCycle                 *lifecycle.Configuration
	NotificationConfiguration notification.Configuration
	Cors                      *cors.Config
	Policy                    string
	ObjectCount               string
	MaxBuckets                string
	BytesUsed                 string
}

func (b *BucketInfo) Make(client *global.GlobalClient) error {

	f, e := CheckBucketExist(client, b.Name)
	if !f {
		return e
	}
	if e != nil {
		return e
	}

	b.Versioning, e = func() (minio.BucketVersioningConfiguration, error) {
		return client.MinioClient.GetBucketVersioning(context.Background(), *b.Name)
	}()
	if e != nil {
		return e
	}

	b.Location, e = func() (string, error) {
		return client.MinioClient.GetBucketLocation(context.Background(), *b.Name)
	}()
	if e != nil {
		return e
	}

	b.LifeCycle, e = func() (*lifecycle.Configuration, error) {
		return client.MinioClient.GetBucketLifecycle(context.Background(), *b.Name)
	}()

	b.NotificationConfiguration, e = func() (notification.Configuration, error) {
		return client.MinioClient.GetBucketNotification(context.Background(), *b.Name)
	}()
	if e != nil {
		return e
	}

	b.Cors, e = func() (*cors.Config, error) {
		return client.MinioClient.GetBucketCors(context.Background(), *b.Name)
	}()
	if e != nil {
		return e
	}

	b.Policy, e = func() (string, error) {
		return client.MinioClient.GetBucketPolicy(context.Background(), *b.Name)
	}()

	stats, e := customoperations.GetRgwBucketStats(client, *b.Name, "")
	if e != nil {
		b.ObjectCount = "undefined"
		b.MaxBuckets = "undefined"
		b.BytesUsed = "undefined"
	} else {
		b.ObjectCount = func() string {
			if count, ok := stats["objectCount"]; ok {
				return count
			}
			return "undefined"
		}()
		b.MaxBuckets = func() string {
			if count, ok := stats["maxBuckets"]; ok {
				return count
			}
			return "undefined"
		}()
		b.BytesUsed = func() string {
			if count, ok := stats["bytesUsed"]; ok {
				return count
			}
			return "undefined"
		}()
	}
	return nil
}

func (b *BucketInfo) ChangeVersioningSettings(client *global.GlobalClient, enable bool) error {
	f, e := CheckBucketExist(client, b.Name)
	if !f {
		return errors.New("bucket not found")
	}
	if e != nil {
		return e
	}
	if enable {
		e = client.MinioClient.EnableVersioning(context.Background(), *b.Name)
		return e
	} else {
		e = client.MinioClient.SuspendVersioning(context.Background(), *b.Name)
		return e
	}
}

func (b *BucketInfo) ChangeBucketPolicy(client *global.GlobalClient, policy string) error {
	f, e := CheckBucketExist(client, b.Name)
	if !f {
		return errors.New("bucket not found")
	}
	if e != nil {
		return e
	}
	// Проводим верификацию json
	if !json.Valid([]byte(policy)) {
		return errors.New("not valid json object")
	}
	return client.MinioClient.SetBucketPolicy(context.Background(), *b.Name, policy)
}

func (b *BucketInfo) ChangeBucketLc(client *global.GlobalClient, config *lifecycle.Configuration) error {
	return client.MinioClient.SetBucketLifecycle(context.Background(), *b.Name, config)
}

func (b *BucketInfo) ChangeNotificationConfig(client *global.GlobalClient, config notification.Configuration) error {
	return client.MinioClient.SetBucketNotification(context.Background(), *b.Name, config)
}
