package bucketoperations

import (
	"context"
	"minioclient/global"

	"github.com/sirupsen/logrus"
)

// Метод проверки существования bucket
// если bucket существует возвращает true иначе false
func CheckBucketExist(client *global.GlobalClient, BucketName *string) (bool, error) {
	found, e := client.MinioClient.BucketExists(context.Background(), *BucketName)
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
