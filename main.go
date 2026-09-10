package main

import (
	"context"
	"flag"
	"io"
	bucketoperations "minioclient/bucket_operations"
	"minioclient/helps"
	objectoperations "minioclient/object_operations"
	"minioclient/tui"
	"minioclient/utils"
	"os"

	"github.com/minio/minio-go/v7"
	"github.com/sirupsen/logrus"
)

// Аргументы запуска
var (
	Examples          *bool
	EndPoint          *string
	Port              *string
	AccessKeyID       *string
	SecretAccessKey   *string
	UseSSL            *bool
	MakeBucket        *bool
	BucketName        *string
	DtsBucketName     *string
	Region            *string
	DeleteBucket      *bool
	ListBuckets       *bool
	ListBucketObjects *bool
	AppendObject      *bool
	PutObject         *bool
	RemoveObject      *bool
	ByLastModify      *bool
	GetObjectStats    *bool
	GetObject         *bool
	ListIncUpl        *bool
	Force             *bool
	Key               *string
	VersionID         *string
	ShowVersions      *bool
	Path              *string
	OutputType        *string
	Debug             *bool
	ObjectTags        *string
	Not               *bool
	DryRun            *bool
	Prefix            *string
	MaxEntrues        *int
	ObjectLocking     *bool
	FixLeak           *bool
	LeakCount         *int
	IndexPool         *string
	ListPeerObject    *int
	ListDirs          *bool
	Interactive       *bool
	Migrate           *bool
	Destination       *string
)

// Глобальная переменная клиента
var Client *minio.Client

// Инициализация флагов запуска
func init() {
	Examples = flag.Bool("examples", false, "print usage examples")
	EndPoint = flag.String("e", "", "host fqdn or ip")
	Port = flag.String("port", "", "api port")
	AccessKeyID = flag.String("accesskey", "", "acess key id")
	SecretAccessKey = flag.String("secretkey", "", "secret key")
	UseSSL = flag.Bool("ssl", true, "use -ssl=false for http connection")
	MakeBucket = flag.Bool("mb", false, "make new bucket")
	BucketName = flag.String("bn", "", "bucket name")
	Region = flag.String("r", "us-east-1", "region")
	DeleteBucket = flag.Bool("db", false, "delete bucket")
	ListBuckets = flag.Bool("lb", false, "list buckets")
	ListBucketObjects = flag.Bool("lbo", false, "list bucket objects")
	AppendObject = flag.Bool("ao", false, "append (update existet) object")
	PutObject = flag.Bool("po", false, "put new object. if object exist update it")
	RemoveObject = flag.Bool("ro", false, "remove object by key")
	ByLastModify = flag.Bool("bylastmodify", false, "Remove bucket object by last modify")
	GetObjectStats = flag.Bool("gos", false, "get object stat (only json out) by key and vid")
	GetObject = flag.Bool("go", false, "get object (only json out) by key and vid")
	ListIncUpl = flag.Bool("liu", false, "list incomplete uploads")
	Force = flag.Bool("f", false, "for supported operation enable force mod")
	Key = flag.String("key", "", "object key")
	Path = flag.String("path", "", "path to file")
	VersionID = flag.String("vid", "", "key file version id for remove operation")
	ShowVersions = flag.Bool("sv", false, "show version files for list bucket objects operation")
	OutputType = flag.String("o", "json", "output type - json, table")
	Debug = flag.Bool("d", false, "print debug messages")
	ObjectTags = flag.String("tags", "", "Object tags")
	Not = flag.Bool("not", false, "operation logick switcher - for -ro -tags")
	DryRun = flag.Bool("dryrun", false, "not apply changes")
	Prefix = flag.String("prefix", "", "Prefix for bucket list object")
	MaxEntrues = flag.Int("maxentry", 1000, "Max list entry - default 1000")
	ObjectLocking = flag.Bool("ol", false, "enable bucket object locking")
	FixLeak = flag.Bool("fixleak", false, "fix leak object in bucket for remove obkject operation")
	LeakCount = flag.Int("leakcount", 10, "threshold is triggered")
	IndexPool = flag.String("indexpool", "default.rgw.buckets.index", "Bucket index pool name")
	ListPeerObject = flag.Int("tpo", 20, "max obkects on table for table printer")
	ListDirs = flag.Bool("ls", false, "List bucket dirs and files")
	Interactive = flag.Bool("i", false, "launche interactive tui")
	Migrate = flag.Bool("migrate", false, "Migrate bucket objects from source endpoint to destination endpoint")
	Destination = flag.String("destination", "", "Destination endpoint for migrate bucket objects")
	DtsBucketName = flag.String("dbn", "", "Destination bucket name for migrate operation")
	flag.Parse()

	// Если дебаг отключен то вывод отбрасываем
	if !*Debug {
		logrus.SetOutput(io.Discard)
	}

	// Печать примеров выполнения
	if *Examples {
		helps.PrintHelpUsage()
		os.Exit(0)
	}

	// Аргумент EndPoint обязателен
	if *EndPoint == "" {
		logrus.Fatal("Endpoint not provided!")
	}

	// Создаем клиента для подключения к кластеру
	var e error
	Client, e = utils.InitClient(EndPoint, Port, AccessKeyID, SecretAccessKey, UseSSL)
	if e != nil {
		logrus.Error("Error init client!")
		os.Exit(1)
	}
}

func main() {
	// Параметры логера
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:    true,
		TimestampFormat:  "2006-01-02 15:04:05.000",
		ForceColors:      true, // Цвета в терминале
		PadLevelText:     true, // Выравнивание уровней
		QuoteEmptyFields: true, // Кавычки для пустых полей
	})
	// Если дебаг отключен то вывод отбрасываем
	if !*Debug {
		logrus.SetOutput(io.Discard)
	}

	// Если не какие аргументы не переданны то запускаем TUI
	if *Interactive {
		tui.App(Client, *Interactive)
		os.Exit(0)
	}

	// Операция миграции бакета из кластера источника в кластер назначения
	if *Migrate && *Destination != "" && *BucketName != "" {
		e := objectoperations.MigrateObjects(Client, *BucketName, *Prefix, *Destination, UseSSL, *MaxEntrues, *DtsBucketName)
		if e != nil {
			logrus.Fatal("Error migrate operation!", e.Error())
		}
		logrus.Info("Succesfule done migrate operation")
		return
	}

	// Создание нового bucket с проверкой что он уже не существует
	if *MakeBucket && *BucketName != "" {
		e := bucketoperations.MakeBucket(Client, BucketName, Region, ObjectLocking)
		if e != nil {
			logrus.Fatal(e)
		}
		return
	}

	// Удаление существющего bucket
	if *DeleteBucket && *BucketName != "" {
		e := bucketoperations.DeleteBucket(Client, BucketName)
		if e != nil {
			logrus.Fatal(e)
		}
		return
	}

	// Просмотр доступных buckets
	if *ListBuckets {
		_, e := bucketoperations.ListBuckets(Client, OutputType)
		if e != nil {
			logrus.Fatal(e)
		}
		return
	}

	if *ListDirs {
		_, e := objectoperations.ListBucketDirs(Client, BucketName, OutputType, Prefix, *Interactive, context.Background())
		if e != nil {
			logrus.Fatal(e)
		}
		return
	}

	// Просмотр объектов в bucket
	if *ListBucketObjects && *BucketName != "" {
		e := objectoperations.ListBucketObjects(Client, BucketName, ShowVersions, OutputType, Prefix, *MaxEntrues, *ListPeerObject)
		if e != nil {
			logrus.Fatal(e)
		}
		return
	}

	// Обновление объекта в bucket
	if *AppendObject && *BucketName != "" && *Path != "" {
		e := objectoperations.AppendBucketObject(Client, Path, BucketName)
		if e != nil {
			logrus.Fatal(e)
		}
		return
	}

	// Добавление объекта в bucket
	if *PutObject && *BucketName != "" && *Path != "" {
		ctx := context.Background()
		e := objectoperations.PutBucketObject(Client, Path, BucketName, ctx, nil, *Prefix, *Interactive)
		if e != nil {
			logrus.Fatal(e)
		}
		return
	}

	// Получение методанных объекта
	if *GetObjectStats && *Key != "" && *BucketName != "" {
		_, e := objectoperations.GetObjectStat(Client, Key, BucketName, VersionID)
		if e != nil {
			logrus.Fatal(e)
		}
		return
	}

	// Получение списка не завершенных задач загрузки
	if *ListIncUpl && *BucketName != "" {
		objectoperations.ListIncompleteUploads(Client, BucketName, OutputType)
		return
	}

	// Метод удаления объекта
	if *RemoveObject {
		// Удаление объектов через lastmodify
		if *BucketName != "" && *ByLastModify {
			e := objectoperations.RmObjectByLastModified(Client, BucketName, Force, *DryRun, *FixLeak, *LeakCount, *IndexPool, *Interactive)
			if e != nil {
				logrus.Fatal(e)
			}
			return

		}
		// Удаление объектов по их тегу
		if *BucketName != "" && *ObjectTags != "" {
			e := objectoperations.RemoveBucketObjectByTags(Client, BucketName, Force, *ObjectTags, *Not, *DryRun, *FixLeak, *LeakCount, *IndexPool, *Interactive)
			if e != nil {
				logrus.Fatal(e)
			}
			return
		}
		// Удаление отдельного объекта по клющу и версии
		if *BucketName != "" && *Key != "" {
			e := objectoperations.RemoveObject(Client, Key, BucketName, Force, VersionID, *DryRun)
			if e != nil {
				logrus.Fatal(e)
			}
			return
		}
		// Массовое удаление объектов из JSON файла
		if *BucketName != "" && *Path != "" && *Key == "" {
			e := objectoperations.RemoveBucketObjects(Client, BucketName, Force, *Path, *DryRun)
			if e != nil {
				logrus.Fatal(e)
			}
			return
		}
	}

	// Получение свойств объекта
	if *GetObject && *Key != "" && *BucketName != "" {
		e := objectoperations.GetObject(Client, Key, BucketName, VersionID)
		if e != nil {
			logrus.Fatal(e)
		}
		return
	}
}
