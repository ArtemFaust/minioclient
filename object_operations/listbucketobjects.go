package objectoperations

import (
	"context"
	"encoding/json"
	"fmt"
	bucketoperations "minioclient/bucket_operations"
	"strings"

	"github.com/fatih/color"
	"github.com/minio/minio-go/v7"
	"github.com/rodaine/table"
	"github.com/sirupsen/logrus"
)

// Метод просмотра объектов в bucket
// возвращет все объекты относчиеся к bucket
func ListBucketObjects(minioClient *minio.Client, BucketName *string, ShowVersions *bool, o *string, prefix *string, maxentry int, tablepeerobjects int) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Проверяем что bucket существует
	f, e := bucketoperations.CheckBucketExist(minioClient, BucketName)
	if e != nil {
		logrus.Error(e)
		return e
	}
	// Если не существует возвращаем ошибку
	if !f {
		e = fmt.Errorf("bucker %s not found", *BucketName)
		logrus.Error(e)
		return e
	}

	/* Получаем все объекты из bucket
	objectCh := minioClient.ListObjects(ctx, *BucketName, minio.ListObjectsOptions{
		Prefix:       *prefix,
		Recursive:    true,
		WithVersions: *ShowVersions,
	})*/

	// Получаем объеккты из бакета
	objectCh := minioClient.ListObjects(ctx, *BucketName, minio.ListObjectsOptions{
		Prefix:       *prefix,
		Recursive:    true,
		WithVersions: *ShowVersions,
	})

	if *o == "table" {
		// Обрабатываем объекты
		otablePrint(objectCh, maxentry, tablepeerobjects)
	}
	if *o == "json" {
		ojsonPrint(objectCh, maxentry, tablepeerobjects)
	}

	return nil
}

// Метод просмотра директорий в bucket
func ListBucketDirs(minioClient *minio.Client, BucketName *string, o *string, prefix *string, interactive bool, ctx context.Context) (<-chan minio.ObjectInfo, error) {
	// Проверяем что bucket существует
	f, e := bucketoperations.CheckBucketExist(minioClient, BucketName)
	if e != nil {
		logrus.Error(e)
		return nil, e
	}

	// Если не существует возвращаем ошибку
	if !f {
		e = fmt.Errorf("bucker %s not found", *BucketName)
		logrus.Error(e)
		return nil, e
	}

	// Получаем объеккты из бакета
	objectCh := minioClient.ListObjects(ctx, *BucketName, minio.ListObjectsOptions{
		Prefix:       *prefix,
		Recursive:    false,
		WithVersions: false,
	})

	reset := "\033[0m"
	blue := "\033[34m"
	//white := "\033[37m"
	yellow := "\033[33m"
	counter := 0

	//objects := []minio.ObjectInfo{}
	if !interactive {
		for o := range objectCh {
			// Если контекст отменен прерываем
			if ctx.Err() != nil {
				break
			}
			counter++
			if strings.HasSuffix(o.Key, "/") {
				key := strings.Split(o.Key, "/")[len(strings.Split(o.Key, "/"))-2] + "/"
				fmt.Println("d", o.Size, blue+key+reset)
			} else {
				key := strings.Split(o.Key, "/")[len(strings.Split(o.Key, "/"))-1]
				fmt.Println("f", o.Size, yellow+key+reset)
			}
		}
		fmt.Println("\ntotal: ", counter)
	} else {
		return objectCh, nil
	}

	return objectCh, nil
}

// Метод печати в табличном виде
func otablePrint(objectCh <-chan minio.ObjectInfo, maxentry int, tablepeerobjects int) {
	headerFmt := color.New(color.FgGreen, color.Underline).SprintfFunc()
	columnFmt := color.New(color.FgYellow).SprintfFunc()

	entry := 0
	buffer := []minio.ObjectInfo{}

	// Обрабатываем объекты
	for object := range objectCh {
		if object.Err != nil {
			logrus.Error(object.Err)
			continue
		}

		entry++
		buffer = append(buffer, object)

		if entry%tablepeerobjects == 0 && entry > 0 {
			tbl := table.New(
				"ETag",
				"Key",
				//"ContentType",
				//"Expiration",
				//"Expires",
				//"ExpirationRuleID",
				//"Grant",
				"IsDeleteMarker",
				"IsLatest",
				"LastModified",
				//"Owner",
				"VersionID",
			)
			tbl.WithHeaderFormatter(headerFmt).WithFirstColumnFormatter(columnFmt)

			for _, o := range buffer {
				tbl.AddRow(o.ETag, // ИСПРАВЛЕНО: было object.ETag, стало o.ETag
					o.Key,
					//o.ContentType,
					//o.Expiration,
					//o.ExpirationRuleID,
					//o.Expires,
					//o.Grant,
					o.IsDeleteMarker,
					o.IsLatest,
					o.LastModified,
					//o.Owner,
					o.VersionID,
				)
			}

			tbl.Print()
			buffer = []minio.ObjectInfo{}
		}

		if entry == maxentry && maxentry != 0 {
			break
		}
	}

	// Выводим оставшиеся объекты в буфере
	if len(buffer) > 0 {
		tbl := table.New(
			"ETag",
			"Key",
			"IsDeleteMarker",
			"IsLatest",
			"LastModified",
			"VersionID",
		)
		tbl.WithHeaderFormatter(headerFmt).WithFirstColumnFormatter(columnFmt)

		for _, o := range buffer {
			tbl.AddRow(o.ETag,
				o.Key,
				o.IsDeleteMarker,
				o.IsLatest,
				o.LastModified,
				o.VersionID,
			)
		}

		tbl.Print()
	}
}

// Метод печати json
func ojsonPrint(objectCh <-chan minio.ObjectInfo, maxentry int, tablepeerobjects int) error {
	oobjects := []minio.ObjectInfo{}
	entry := 0
	for object := range objectCh {
		if object.Err != nil {
			logrus.Error(object.Err)
			continue
		}

		oobjects = append(oobjects, object)
		entry++

		if entry%tablepeerobjects == 0 && entry > 0 {
			b, e := json.MarshalIndent(oobjects, "", "  ")
			if e != nil {
				logrus.Error(e)
				return e
			}
			fmt.Println(string(b))
			oobjects = []minio.ObjectInfo{}
		}

		if entry == maxentry && maxentry != 0 {
			break
		}
	}
	return nil
}
