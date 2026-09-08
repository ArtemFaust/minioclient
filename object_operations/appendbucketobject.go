package objectoperations

import (
	"bufio"
	"context"
	"os"

	"github.com/minio/minio-go/v7"
	"github.com/sirupsen/logrus"
)

// Метод обновление данных в существующем объекте
func AppendBucketObject(minioClient *minio.Client, Path *string, BucketName *string) error {
	opt := minio.AppendObjectOptions{}
	f, e := os.Open(*Path)
	if e != nil {
		logrus.Error(e)
		return e
	}
	defer f.Close()
	f_stat, e := f.Stat()
	if e != nil {
		logrus.Error(e)
		return e
	}
	// Загрузка отдельного файла
	if !f_stat.IsDir() {
		logrus.Info("appended file name: ", f_stat.Name(), " size: ", f_stat.Size(), " bytes")
		info, e := minioClient.AppendObject(
			context.Background(),
			*BucketName,
			f_stat.Name(),
			bufio.NewReader(f),
			f_stat.Size(),
			opt,
		)
		if e != nil {
			logrus.Error(e)
			return e
		}
		logrus.Info(info)
	}
	return nil
}
