package objectoperations

import (
	"context"
	"fmt"
	"io/fs"
	"minioclient/global"
	"os"
	"path/filepath"
	"runtime"

	"github.com/cheggaaa/pb"
	"github.com/minio/minio-go/v7"
	"github.com/sirupsen/logrus"
)

// Метод создания нового объекта
// ( обновление если объект существует создавая версию файла)
func PutBucketObject(client *global.GlobalClient, Path *string, BucketName *string,
	ctx context.Context, ch chan string, prefix string, interactive bool) error {
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

		// Временно отключил мультипарт так как ошибка md5 check sum...
		var options minio.PutObjectOptions
		if !interactive {
			// Прогресс операции
			progress := pb.New64(f_stat.Size())
			progress.Start()
			options = minio.PutObjectOptions{
				ContentType:           "application/octet-stream",
				Progress:              progress,
				NumThreads:            uint(runtime.NumCPU()),                 // Кол-во поток загрузки объекта в бакет по кол-ву CPU
				ConcurrentStreamParts: client.ClientCfg.ConcurrentStreamParts, // Конкурентная загрузка
				DisableMultipart:      client.ClientCfg.DisableMultipart,
				Checksum: func() minio.ChecksumType {
					// Если в конфиге маультипарт отключен то хешсумму не читаем
					if client.ClientCfg.DisableMultipart {
						return minio.ChecksumNone
					}
					// Если не указан тип хеша то используем по умолчанию ChecksumCRC64NVME
					if client.ClientCfg.MultipartChecksSum == "" {
						return minio.ChecksumCRC64NVME
					}
					// ChecksumCRC32|ChecksumCRC32C|ChecksumCRC64NVME|ChecksumMD5|ChecksumSHA1|ChecksumSHA256|ChecksumSHA512|ChecksumXXHASH3|ChecksumXXHASH64
					switch client.ClientCfg.MultipartChecksSum {
					case "ChecksumCRC32":
						return minio.ChecksumCRC32
					case "ChecksumCRC32C":
						return minio.ChecksumCRC32
					case "ChecksumCRC64NVME":
						return minio.ChecksumCRC64NVME
					case "ChecksumMD5":
						return minio.ChecksumMD5
					case "ChecksumSHA1":
						return minio.ChecksumSHA1
					case "ChecksumSHA256":
						return minio.ChecksumSHA256
					case "ChecksumSHA512":
						return minio.ChecksumSHA512
					case "ChecksumXXHASH3":
						return minio.ChecksumXXHash3
					case "ChecksumXXHASH64":
						return minio.ChecksumXXHash64
					case "ChecksumNone":
						return minio.ChecksumNone
					default:
						return minio.ChecksumCRC64NVME
					}
				}(),
				AutoChecksum: func() minio.ChecksumType {
					// Если в конфиге маультипарт отключен то хешсумму не читаем
					if client.ClientCfg.DisableMultipart {
						return minio.ChecksumNone
					}
					// Если не указан тип хеша то используем по умолчанию ChecksumCRC64NVME
					if client.ClientCfg.MultipartChecksSum == "" {
						return minio.ChecksumCRC64NVME
					}
					// ChecksumCRC32|ChecksumCRC32C|ChecksumCRC64NVME|ChecksumMD5|ChecksumSHA1|ChecksumSHA256|ChecksumSHA512|ChecksumXXHASH3|ChecksumXXHASH64
					switch client.ClientCfg.MultipartChecksSum {
					case "ChecksumCRC32":
						return minio.ChecksumCRC32
					case "ChecksumCRC32C":
						return minio.ChecksumCRC32
					case "ChecksumCRC64NVME":
						return minio.ChecksumCRC64NVME
					case "ChecksumMD5":
						return minio.ChecksumMD5
					case "ChecksumSHA1":
						return minio.ChecksumSHA1
					case "ChecksumSHA256":
						return minio.ChecksumSHA256
					case "ChecksumSHA512":
						return minio.ChecksumSHA512
					case "ChecksumXXHASH3":
						return minio.ChecksumXXHash3
					case "ChecksumXXHASH64":
						return minio.ChecksumXXHash64
					case "ChecksumNone":
						return minio.ChecksumNone
					default:
						return minio.ChecksumCRC64NVME
					}
				}(),
				SendContentMd5: client.ClientCfg.SendMd5CheckSum,
			}
		} else {
			options = minio.PutObjectOptions{
				ContentType:           "application/octet-stream",
				NumThreads:            uint(runtime.NumCPU()),                 // Кол-во поток загрузки объекта в бакет по кол-ву CPU
				ConcurrentStreamParts: client.ClientCfg.ConcurrentStreamParts, // Конкурентная загрузка
				DisableMultipart:      client.ClientCfg.DisableMultipart,
				Checksum: func() minio.ChecksumType {
					// Если в конфиге маультипарт отключен то хешсумму не читаем
					if client.ClientCfg.DisableMultipart {
						return minio.ChecksumNone
					}
					// Если не указан тип хеша то используем по умолчанию ChecksumCRC64NVME
					if client.ClientCfg.MultipartChecksSum == "" {
						return minio.ChecksumCRC64NVME
					}
					// ChecksumCRC32|ChecksumCRC32C|ChecksumCRC64NVME|ChecksumMD5|ChecksumSHA1|ChecksumSHA256|ChecksumSHA512|ChecksumXXHASH3|ChecksumXXHASH64
					switch client.ClientCfg.MultipartChecksSum {
					case "ChecksumCRC32":
						return minio.ChecksumCRC32
					case "ChecksumCRC32C":
						return minio.ChecksumCRC32
					case "ChecksumCRC64NVME":
						return minio.ChecksumCRC64NVME
					case "ChecksumMD5":
						return minio.ChecksumMD5
					case "ChecksumSHA1":
						return minio.ChecksumSHA1
					case "ChecksumSHA256":
						return minio.ChecksumSHA256
					case "ChecksumSHA512":
						return minio.ChecksumSHA512
					case "ChecksumXXHASH3":
						return minio.ChecksumXXHash3
					case "ChecksumXXHASH64":
						return minio.ChecksumXXHash64
					case "ChecksumNone":
						return minio.ChecksumNone
					default:
						return minio.ChecksumCRC64NVME
					}
				}(),
				AutoChecksum: func() minio.ChecksumType {
					// Если в конфиге маультипарт отключен то хешсумму не читаем
					if client.ClientCfg.DisableMultipart {
						return minio.ChecksumNone
					}
					// Если не указан тип хеша то используем по умолчанию ChecksumCRC64NVME
					if client.ClientCfg.MultipartChecksSum == "" {
						return minio.ChecksumCRC64NVME
					}
					// ChecksumCRC32|ChecksumCRC32C|ChecksumCRC64NVME|ChecksumMD5|ChecksumSHA1|ChecksumSHA256|ChecksumSHA512|ChecksumXXHASH3|ChecksumXXHASH64
					switch client.ClientCfg.MultipartChecksSum {
					case "ChecksumCRC32":
						return minio.ChecksumCRC32
					case "ChecksumCRC32C":
						return minio.ChecksumCRC32
					case "ChecksumCRC64NVME":
						return minio.ChecksumCRC64NVME
					case "ChecksumMD5":
						return minio.ChecksumMD5
					case "ChecksumSHA1":
						return minio.ChecksumSHA1
					case "ChecksumSHA256":
						return minio.ChecksumSHA256
					case "ChecksumSHA512":
						return minio.ChecksumSHA512
					case "ChecksumXXHASH3":
						return minio.ChecksumXXHash3
					case "ChecksumXXHASH64":
						return minio.ChecksumXXHash64
					case "ChecksumNone":
						return minio.ChecksumNone
					default:
						return minio.ChecksumCRC64NVME
					}
				}(),
				SendContentMd5: client.ClientCfg.SendMd5CheckSum,
			}
		}

		uploadInfo, e := client.MinioClient.PutObject(
			ctx,
			*BucketName,
			func() string {
				if prefix != "" {
					return prefix + f_stat.Name()
				} else {
					return f_stat.Name()
				}
			}(),
			f,
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

				// Если объект маленький то принудительно отключаем мультипарт загрузку для него
				if f_stat.Size() < 15728640 {
					client.ClientCfg.DisableMultipart = true
				}

				// Временно отключил мультипарт так как ошибка md5 check sum...
				var options minio.PutObjectOptions
				if !interactive {
					// Прогресс операции
					progress := pb.New64(f_stat.Size())
					progress.Start()
					options = minio.PutObjectOptions{
						ContentType:           "application/octet-stream",
						Progress:              progress,
						NumThreads:            uint(runtime.NumCPU()),                 // Кол-во поток загрузки объекта в бакет по кол-ву CPU
						ConcurrentStreamParts: client.ClientCfg.ConcurrentStreamParts, // Конкурентная загрузка
						DisableMultipart:      client.ClientCfg.DisableMultipart,
						Checksum: func() minio.ChecksumType {
							// Если в конфиге маультипарт отключен то хешсумму не читаем
							if client.ClientCfg.DisableMultipart {
								return minio.ChecksumNone
							}
							// Если не указан тип хеша то используем по умолчанию ChecksumCRC64NVME
							if client.ClientCfg.MultipartChecksSum == "" {
								return minio.ChecksumCRC64NVME
							}
							// ChecksumCRC32|ChecksumCRC32C|ChecksumCRC64NVME|ChecksumMD5|ChecksumSHA1|ChecksumSHA256|ChecksumSHA512|ChecksumXXHASH3|ChecksumXXHASH64
							switch client.ClientCfg.MultipartChecksSum {
							case "ChecksumCRC32":
								return minio.ChecksumCRC32
							case "ChecksumCRC32C":
								return minio.ChecksumCRC32
							case "ChecksumCRC64NVME":
								return minio.ChecksumCRC64NVME
							case "ChecksumMD5":
								return minio.ChecksumMD5
							case "ChecksumSHA1":
								return minio.ChecksumSHA1
							case "ChecksumSHA256":
								return minio.ChecksumSHA256
							case "ChecksumSHA512":
								return minio.ChecksumSHA512
							case "ChecksumXXHASH3":
								return minio.ChecksumXXHash3
							case "ChecksumXXHASH64":
								return minio.ChecksumXXHash64
							case "ChecksumNone":
								return minio.ChecksumNone
							default:
								return minio.ChecksumCRC64NVME
							}
						}(),
						AutoChecksum: func() minio.ChecksumType {
							// Если в конфиге маультипарт отключен то хешсумму не читаем
							if client.ClientCfg.DisableMultipart {
								return minio.ChecksumNone
							}
							// Если не указан тип хеша то используем по умолчанию ChecksumCRC64NVME
							if client.ClientCfg.MultipartChecksSum == "" {
								return minio.ChecksumCRC64NVME
							}
							// ChecksumCRC32|ChecksumCRC32C|ChecksumCRC64NVME|ChecksumMD5|ChecksumSHA1|ChecksumSHA256|ChecksumSHA512|ChecksumXXHASH3|ChecksumXXHASH64
							switch client.ClientCfg.MultipartChecksSum {
							case "ChecksumCRC32":
								return minio.ChecksumCRC32
							case "ChecksumCRC32C":
								return minio.ChecksumCRC32
							case "ChecksumCRC64NVME":
								return minio.ChecksumCRC64NVME
							case "ChecksumMD5":
								return minio.ChecksumMD5
							case "ChecksumSHA1":
								return minio.ChecksumSHA1
							case "ChecksumSHA256":
								return minio.ChecksumSHA256
							case "ChecksumSHA512":
								return minio.ChecksumSHA512
							case "ChecksumXXHASH3":
								return minio.ChecksumXXHash3
							case "ChecksumXXHASH64":
								return minio.ChecksumXXHash64
							case "ChecksumNone":
								return minio.ChecksumNone
							default:
								return minio.ChecksumCRC64NVME
							}
						}(),
						SendContentMd5: client.ClientCfg.SendMd5CheckSum,
					}
				} else {
					options = minio.PutObjectOptions{
						ContentType:           "application/octet-stream",
						NumThreads:            uint(runtime.NumCPU()),                 // Кол-во поток загрузки объекта в бакет по кол-ву CPU
						ConcurrentStreamParts: client.ClientCfg.ConcurrentStreamParts, // Конкурентная загрузка
						DisableMultipart:      client.ClientCfg.DisableMultipart,
						Checksum: func() minio.ChecksumType {
							// Если в конфиге маультипарт отключен то хешсумму не читаем
							if client.ClientCfg.DisableMultipart {
								return minio.ChecksumNone
							}
							// Если не указан тип хеша то используем по умолчанию ChecksumCRC64NVME
							if client.ClientCfg.MultipartChecksSum == "" {
								return minio.ChecksumCRC64NVME
							}
							// ChecksumCRC32|ChecksumCRC32C|ChecksumCRC64NVME|ChecksumMD5|ChecksumSHA1|ChecksumSHA256|ChecksumSHA512|ChecksumXXHASH3|ChecksumXXHASH64
							switch client.ClientCfg.MultipartChecksSum {
							case "ChecksumCRC32":
								return minio.ChecksumCRC32
							case "ChecksumCRC32C":
								return minio.ChecksumCRC32
							case "ChecksumCRC64NVME":
								return minio.ChecksumCRC64NVME
							case "ChecksumMD5":
								return minio.ChecksumMD5
							case "ChecksumSHA1":
								return minio.ChecksumSHA1
							case "ChecksumSHA256":
								return minio.ChecksumSHA256
							case "ChecksumSHA512":
								return minio.ChecksumSHA512
							case "ChecksumXXHASH3":
								return minio.ChecksumXXHash3
							case "ChecksumXXHASH64":
								return minio.ChecksumXXHash64
							case "ChecksumNone":
								return minio.ChecksumNone
							default:
								return minio.ChecksumCRC64NVME
							}
						}(),
						AutoChecksum: func() minio.ChecksumType {
							// Если в конфиге маультипарт отключен то хешсумму не читаем
							if client.ClientCfg.DisableMultipart {
								return minio.ChecksumNone
							}
							// Если не указан тип хеша то используем по умолчанию ChecksumCRC64NVME
							if client.ClientCfg.MultipartChecksSum == "" {
								return minio.ChecksumCRC64NVME
							}
							// ChecksumCRC32|ChecksumCRC32C|ChecksumCRC64NVME|ChecksumMD5|ChecksumSHA1|ChecksumSHA256|ChecksumSHA512|ChecksumXXHASH3|ChecksumXXHASH64
							switch client.ClientCfg.MultipartChecksSum {
							case "ChecksumCRC32":
								return minio.ChecksumCRC32
							case "ChecksumCRC32C":
								return minio.ChecksumCRC32
							case "ChecksumCRC64NVME":
								return minio.ChecksumCRC64NVME
							case "ChecksumMD5":
								return minio.ChecksumMD5
							case "ChecksumSHA1":
								return minio.ChecksumSHA1
							case "ChecksumSHA256":
								return minio.ChecksumSHA256
							case "ChecksumSHA512":
								return minio.ChecksumSHA512
							case "ChecksumXXHASH3":
								return minio.ChecksumXXHash3
							case "ChecksumXXHASH64":
								return minio.ChecksumXXHash64
							case "ChecksumNone":
								return minio.ChecksumNone
							default:
								return minio.ChecksumCRC64NVME
							}
						}(),
						SendContentMd5: client.ClientCfg.SendMd5CheckSum,
					}
				}

				// Загружаем фаил
				uploadInfo, e := client.MinioClient.PutObject(
					ctx,
					*BucketName,
					func() string {
						if prefix != "" {
							return prefix + path
						} else {
							return path
						}
					}(),
					f,
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
