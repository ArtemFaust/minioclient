package objectoperations

import (
	"bufio"
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/minio/minio-go/v7"
	"github.com/sirupsen/logrus"
)

func GetObject(minioClient *minio.Client, Key *string, BucketName *string, versionid *string) error {
	obj, e := minioClient.GetObject(context.Background(), *BucketName, *Key, minio.GetObjectOptions{
		VersionID: *versionid,
		Checksum:  true,
	})

	if e != nil {
		logrus.Error(e)
		return e
	}
	defer obj.Close()

	file, err := createFileWithDirs(*Key)
	if err != nil {
		return err
	}
	defer file.Close()

	// Записываем в фаил
	bufferReader := bufio.NewReader(obj)
	bufferWriter := bufio.NewWriter(file)
	defer bufferWriter.Flush()
	_, e = io.Copy(bufferWriter, bufferReader)

	return e
}

func createFileWithDirs(filePath string) (*os.File, error) {
	// Получаем директорию из пути
	dir := filepath.Dir(filePath)

	// Создаем все необходимые директории
	if dir != "." && dir != "" {
		if e := os.MkdirAll(dir, 0755); e != nil {
			return nil, e
		}
	}

	// Создаем файл
	file, e := os.Create(filePath)
	if e != nil {
		return nil, e
	}

	return file, nil
}
