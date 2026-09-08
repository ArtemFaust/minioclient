package objectoperations

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/minio/minio-go/v7"
	"github.com/sirupsen/logrus"
)

// Метод получения методанных объекта
// возвращает методанные указанног ообъекта
func GetObjectStat(minioClient *minio.Client, Key *string, BucketName *string, versionid *string) (string, error) {
	objInfo, e := minioClient.StatObject(context.Background(), *BucketName, *Key, minio.StatObjectOptions{
		VersionID: *versionid,
		Checksum:  true,
	})

	if e != nil {
		logrus.Error(e)
		return "", e
	}

	s, e := jsonPrinter(objInfo)
	if e != nil {
		logrus.Error(e)
		return "", e
	}
	return s, nil
}

func jsonPrinter(objInfo minio.ObjectInfo) (string, error) {
	b, e := json.MarshalIndent(objInfo, "", "  ")
	if e != nil {
		return "", e
	}
	fmt.Println(string(b))
	return string(b), nil
}
