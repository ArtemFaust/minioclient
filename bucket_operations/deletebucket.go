package bucketoperations

import (
	"context"
	"fmt"

	"github.com/minio/minio-go/v7"
	"github.com/sirupsen/logrus"
)

// Метод удления bucket
// удаление возможно только bucket в котором
// нет файлов
// при удаление не пустого bucket возникнет ошибка
func DeleteBucket(minioClient *minio.Client, BucketName *string) error {
	f, e := CheckBucketExist(minioClient, BucketName)
	if e != nil {
		logrus.Error(e)
		return e
	}
	if !f {
		logrus.Error("bucket ", *BucketName, " not found")
		return fmt.Errorf("bucket %s not found", *BucketName)
	}
	e = minioClient.RemoveBucket(context.Background(), *BucketName)
	if e != nil {
		logrus.Error(e)
		return e
	}
	logrus.Info("succesfule delete bucket: ", *BucketName)
	return nil
}
