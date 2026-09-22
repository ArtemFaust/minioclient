package bucketoperations

import (
	"context"
	"encoding/json"
	"errors"
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
}

func (b *BucketInfo) Make(client *global.GlobalClient) {
	f, e := CheckBucketExist(client, b.Name)
	if !f {
		return
	}
	if e != nil {
		return
	}
	b.Versioning = func() minio.BucketVersioningConfiguration {
		v, _ := client.MinioClient.GetBucketVersioning(context.Background(), *b.Name)
		return v
	}()

	b.Location = func() string {
		location, _ := client.MinioClient.GetBucketLocation(context.Background(), *b.Name)
		return location
	}()

	b.LifeCycle = func() *lifecycle.Configuration {
		lc, _ := client.MinioClient.GetBucketLifecycle(context.Background(), *b.Name)
		return lc
	}()

	b.NotificationConfiguration = func() notification.Configuration {
		notif, _ := client.MinioClient.GetBucketNotification(context.Background(), *b.Name)
		return notif
	}()

	b.Cors = func() *cors.Config {
		cors, _ := client.MinioClient.GetBucketCors(context.Background(), *b.Name)
		return cors
	}()

	b.Policy = func() string {
		policy, _ := client.MinioClient.GetBucketPolicy(context.Background(), *b.Name)
		return policy
	}()
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
