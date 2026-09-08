package bucketoperations

import (
	"context"

	"github.com/minio/minio-go/v7"
	"github.com/sirupsen/logrus"
)

// Метод проверки существования bucket
// если bucket существует возвращает true иначе false
func CheckBucketExist(minioClient *minio.Client, BucketName *string) (bool, error) {
	found, e := minioClient.BucketExists(context.Background(), *BucketName)
	if e != nil {
		logrus.Error(e)
		return false, e
	}
	if found {
		logrus.Info("bucket ", *BucketName, " exist status ", found)
	} else {
		logrus.Warn("bucket ", *BucketName, " exist status ", found)
	}

	return found, nil
}
