package bucketoperations

import (
	"context"
	"encoding/json"
	"errors"

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
}

func (b *BucketInfo) Make(minioClient *minio.Client) {
	f, e := CheckBucketExist(minioClient, b.Name)
	if !f {
		return
	}
	if e != nil {
		return
	}
	b.Versioning = func() minio.BucketVersioningConfiguration {
		v, _ := minioClient.GetBucketVersioning(context.Background(), *b.Name)
		return v
	}()

	b.Location = func() string {
		location, _ := minioClient.GetBucketLocation(context.Background(), *b.Name)
		return location
	}()

	b.LifeCycle = func() *lifecycle.Configuration {
		lc, _ := minioClient.GetBucketLifecycle(context.Background(), *b.Name)
		return lc
	}()

	b.NotificationConfiguration = func() notification.Configuration {
		notif, _ := minioClient.GetBucketNotification(context.Background(), *b.Name)
		return notif
	}()

	b.Cors = func() *cors.Config {
		cors, _ := minioClient.GetBucketCors(context.Background(), *b.Name)
		return cors
	}()

	b.Policy = func() string {
		policy, _ := minioClient.GetBucketPolicy(context.Background(), *b.Name)
		return policy
	}()
}

func (b *BucketInfo) ChangeVersioningSettings(minioClient *minio.Client, enable bool) error {
	f, e := CheckBucketExist(minioClient, b.Name)
	if !f {
		return errors.New("bucket not found")
	}
	if e != nil {
		return e
	}
	if enable {
		e = minioClient.EnableVersioning(context.Background(), *b.Name)
		return e
	} else {
		e = minioClient.SuspendVersioning(context.Background(), *b.Name)
		return e
	}
}

func (b *BucketInfo) ChangeBucketPolicy(minioClient *minio.Client, policy string) error {
	f, e := CheckBucketExist(minioClient, b.Name)
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
	return minioClient.SetBucketPolicy(context.Background(), *b.Name, policy)
}

func (b *BucketInfo) ChangeBucketLc(minioClient *minio.Client, config *lifecycle.Configuration) error {
	return minioClient.SetBucketLifecycle(context.Background(), *b.Name, config)
}

func (b *BucketInfo) ChangeNotificationConfig(minioClient *minio.Client, config notification.Configuration) error {
	return minioClient.SetBucketNotification(context.Background(), *b.Name, config)
}
