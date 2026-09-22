package bucketoperations

import (
	"context"
	"encoding/json"
	"fmt"
	"minioclient/global"

	"github.com/fatih/color"
	"github.com/minio/minio-go/v7"
	"github.com/rodaine/table"
	"github.com/sirupsen/logrus"
)

// Метод получения списка доступных bucket
func ListBuckets(client *global.GlobalClient, o *string) ([]minio.BucketInfo, error) {
	buckets, e := client.MinioClient.ListBuckets(context.Background())
	if e != nil {
		logrus.Error(e)
		return buckets, e
	}
	if o != nil {
		if *o == "table" {
			btablePrint(buckets)
		}
		if *o == "json" {
			bjsonPrinter(buckets)
		}
	}

	return buckets, nil
}

// Метод печати в табличном виде
func btablePrint(buckets []minio.BucketInfo) {
	headerFmt := color.New(color.FgGreen, color.Underline).SprintfFunc()
	columnFmt := color.New(color.FgYellow).SprintfFunc()

	tbl := table.New("Name", "CreationDate", "BucketRegion")
	tbl.WithHeaderFormatter(headerFmt).WithFirstColumnFormatter(columnFmt)

	for _, bucket := range buckets {
		tbl.AddRow(bucket.Name, bucket.CreationDate, bucket.BucketRegion)
	}

	tbl.Print()
}

// Метод печати в json
func bjsonPrinter(buckets []minio.BucketInfo) error {
	b, e := json.MarshalIndent(buckets, "", "  ")
	if e != nil {
		logrus.Error(e)
		return e
	}
	fmt.Println(string(b))
	return nil
}
