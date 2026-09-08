package objectoperations

import (
	"bufio"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/cheggaaa/pb"
	"github.com/minio/minio-go/v7"
	"github.com/sirupsen/logrus"
)

// Метод создания нового объекта
// ( обновление если объект существует создавая версию файла)
func PutBucketObject(minioClient *minio.Client, Path *string, BucketName *string, ctx context.Context, ch chan string, prefix string, interactive bool) error {
	// Открываем фаил
	f, e := os.Open(*Path)
	if e != nil {
		logrus.Error(e)
		return e
	}
	defer f.Close()

	// Получаем stat информацию о файле
	f_stat, e := f.Stat()
	if e != nil {
		logrus.Error(e)
		return e
	}

	// Загрузка отдельного файла
	if !f_stat.IsDir() {
		logrus.Info("put file name: ", f_stat.Name(), " size: ", f_stat.Size(), " bytes")

		var options minio.PutObjectOptions
		if !interactive {
			// Прогресс операции
			progress := pb.New64(f_stat.Size())
			progress.Start()
			options = minio.PutObjectOptions{
				ContentType: "application/octet-stream",
				Progress:    progress,
			}
		} else {
			options = minio.PutObjectOptions{
				ContentType: "application/octet-stream",
			}
		}

		uploadInfo, e := minioClient.PutObject(
			ctx,
			*BucketName,
			func() string {
				if prefix != "" {
					return prefix + f_stat.Name()
				} else {
					return f_stat.Name()
				}
			}(),
			bufio.NewReader(f),
			f_stat.Size(),
			options,
		)

		if e != nil {
			logrus.Error(e)
			return e
		}

		logrus.Info("Successfully uploaded file: ", uploadInfo.Key, " etag: ", uploadInfo.ETag, " bytes: ", uploadInfo.Size)
		if ch != nil {
			ch <- fmt.Sprintf("Successfully uploaded file: %s ", uploadInfo.Key)
		}
	} else {
		// Загрузка директории
		e = filepath.WalkDir(*Path, func(path string, d fs.DirEntry, e error) error {
			if e != nil {
				// Если ошибка доступа к файлу то останавливаем выполнение
				logrus.Error(e)
				return e
			}

			// Если контекст отменен то останавливаем выполнение
			if e := ctx.Err(); e != nil {
				logrus.Info("Контекст загрузки отменен")
				return nil
			}

			// Если не директория то загружаем
			if !d.IsDir() {
				f, e := os.Open(path)
				if e != nil {
					logrus.Error(e)
					return e
				}
				defer f.Close()

				// Получаем stat информацию о файле
				f_stat, e := f.Stat()
				if e != nil {
					logrus.Error(e)
					return e
				}
				var options minio.PutObjectOptions
				if !interactive {
					// Прогресс операции
					progress := pb.New64(f_stat.Size())
					progress.Start()
					options = minio.PutObjectOptions{
						ContentType: "application/octet-stream",
						Progress:    progress,
					}
				} else {
					options = minio.PutObjectOptions{
						ContentType: "application/octet-stream",
					}
				}
				// Загружаем фаил
				uploadInfo, e := minioClient.PutObject(
					ctx,
					*BucketName,
					func() string {
						if prefix != "" {
							return prefix + path
						} else {
							return path
						}
					}(),
					bufio.NewReader(f),
					f_stat.Size(),
					options,
				)
				if e != nil {
					logrus.Error(e)
					return e
				}
				logrus.Info("Successfully uploaded file: ", uploadInfo.Key, " etag: ", uploadInfo.ETag, " bytes: ", uploadInfo.Size)
				if ch != nil {
					ch <- fmt.Sprintf("Successfully uploaded file: %s ", uploadInfo.Key)
				}
			}
			return nil
		})
		if e != nil {
			return e
		}
	}

	return nil
}
