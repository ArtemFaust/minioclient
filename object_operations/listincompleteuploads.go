package objectoperations

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/fatih/color"
	"github.com/minio/minio-go/v7"
	"github.com/rodaine/table"
	"github.com/sirupsen/logrus"
)

// Метод просмотра текучих задач по загрузке
// возвращает массив объектов которые
// еще не загрузись на сервер и находятся в статусе загрузки
func ListIncompleteUploads(minioClient *minio.Client, BucketName *string, o *string) {
	isRecursive := true // Recursively list
	multiPartObjectCh := minioClient.ListIncompleteUploads(context.Background(), *BucketName, "/", isRecursive)
	if *o == "table" {
		ltablePrint(multiPartObjectCh)
	}
	if *o == "json" {
		ljsonPrint(multiPartObjectCh)
	}
}

// Метод табличной печати
func ltablePrint(multiPartObjectCh <-chan minio.ObjectMultipartInfo) {
	headerFmt := color.New(color.FgGreen, color.Underline).SprintfFunc()
	columnFmt := color.New(color.FgYellow).SprintfFunc()

	tbl := table.New("UploadID", "Key", "Size")
	tbl.WithHeaderFormatter(headerFmt).WithFirstColumnFormatter(columnFmt)
	for multiPartObject := range multiPartObjectCh {
		if multiPartObject.Err != nil {
			logrus.Error(multiPartObject.Err)
			continue
		}
		tbl.AddRow(multiPartObject.UploadID, multiPartObject.Key, multiPartObject.Size)
	}
	tbl.Print()
}

// Метод печати json
func ljsonPrint(multiPartObjectCh <-chan minio.ObjectMultipartInfo) error {
	objects := []minio.ObjectMultipartInfo{}
	for multiPartObject := range multiPartObjectCh {
		if multiPartObject.Err != nil {
			logrus.Error(multiPartObject.Err)
			continue
		}
		objects = append(objects, multiPartObject)
	}

	b, e := json.MarshalIndent(objects, "", "  ")
	if e != nil {
		logrus.Error(e)
		return e
	}
	fmt.Println(string(b))
	return nil
}
