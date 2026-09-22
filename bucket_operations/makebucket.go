package bucketoperations

import (
	"context"
	"errors"
	"minioclient/global"

	"github.com/minio/minio-go/v7"
	"github.com/sirupsen/logrus"
)

// Метод создания нового bucket
// сщздает новый bucket с указанным именем
// если bucket существет то возврашет ошибку
func MakeBucket(client *global.GlobalClient, BucketName *string, Region *string, ol *bool) error {
	f, e := CheckBucketExist(client, BucketName)
	if e != nil {
		return e
	}
	if f {
		logrus.Error("bucket ", *BucketName, " alredi exist")
		return errors.New("bucket alredi exist")
	}
	e = client.MinioClient.MakeBucket(
		context.Background(),
		*BucketName,
		minio.MakeBucketOptions{Region: *Region, ObjectLocking: *ol},
	)
	if e != nil {
		logrus.Error(e)
		return e
	}
	logrus.Info("create bucket ", *BucketName, " success")
	return nil
}
