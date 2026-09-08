package main

import (
	"context"
	"errors"
	"flag"
	"io"
	bucketoperations "minioclient/bucket_operations"
	"minioclient/global"
	"minioclient/helps"
	objectoperations "minioclient/object_operations"
	"minioclient/tui"
	"minioclient/utils"
	"os"

	"github.com/ghodss/yaml"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
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
	MaxEntrues = flag.Int("maxentry", 0, "Max list entry - default 1000")
	ObjectLocking = flag.Bool("ol", false, "enable bucket object locking")
	FixLeak = flag.Bool("fixleak", false, "fix leak object in bucket for remove obkject operation")
	LeakCount = flag.Int("leakcount", 10, "threshold is triggered")
	IndexPool = flag.String("indexpool", "default.rgw.buckets.index", "Bucket index pool name")
	ListPeerObject = flag.Int("tpo", 20, "max obkects on table for table printer")
	ListDirs = flag.Bool("ls", false, "List bucket dirs and files")
	Interactive = flag.Bool("i", false, "launche interactive tui")
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

	// Снача ищем данное подключение в конфигурации
	// Считывание конфигурационного файла
	cfg, e := readcfg()
	// Если воникла ошибка чтения конфигурации тогда
	// Передаваемый endpoint считаем не как имя в конфигурации - а как fqdn узла к которому подключемся
	// СЛОЖНА БЛЯТЬ СЛОЖНА НИ ХУЯ НЕ ПОНЯТНО
	if e != nil {
		logrus.Warn("Error read cfg file: ", e)
		// Если конфигурацию прочитать не удалось то подразумеваем
		// что передан внешний endpoint и нуждны доп аргументы для подключения
		if *Port == "" || *AccessKeyID == "" || *SecretAccessKey == "" {
			logrus.Error("Please provide port access key id and secret key!")
			os.Exit(1)
		}
		Client, e = makeClient()
		if e != nil {
			logrus.Error("Failed to create client!")
			os.Exit(1)
		}
		// Если конфигурацию нашли и ее удалось прочитать
	} else {
		// Проверяем есть ли переданный endpoint в конфигурации
		for i, connection := range cfg.Connections {
			// Если находим параметры в конфигурации то используем их
			if connection.Name == *EndPoint {
				logrus.Info("Found configuration for endpoint: ", *EndPoint)
				// Основные парамепптры подключения
				Port = &cfg.Connections[i].Port
				AccessKeyID = &cfg.Connections[i].Acesskey
				SecretAccessKey = &cfg.Connections[i].Secretkey

				// Выбор точки подключения для найденного endpoint
				for y, endpoint := range cfg.Connections[i].Endpoints {
					// Пробуем подключить к выбранному клиенту
					EndPoint = &cfg.Connections[i].Endpoints[y]
					logrus.Info("Selected endpoint: ", *EndPoint+":"+*Port)
					Client, e = makeClient() // Пытаемся установить тестовое подключение
					if e != nil {
						logrus.Error("Error connect to endpoint: ", endpoint+":"+*Port, " Error: ", e)
						if len(cfg.Connections[i].Endpoints)-1 == i {
							logrus.Error("All selected endpoints not available!")
							os.Exit(1)
						}
						continue
					}
					return
				}
			}
		}
		logrus.Warn("Not found configuration for endpoint: ", *EndPoint)
		// Если в конфигурации не нашли нужного подключения то подразумеваем
		// что передан внешний endpoint и нуждны доп аргументы для подключения
		if *Port == "" || *AccessKeyID == "" || *SecretAccessKey == "" {
			logrus.Error("Please provide port access key id and secret key!")
			os.Exit(1)
		}
		Client, e = makeClient()
		if e != nil {
			logrus.Error("Failed to create client!")
			os.Exit(1)
		}
	}
}

func main() {
	// Если не какие аргументы не переданны то запускаем TUI
	if *Interactive {
		tui.App(Client, *Interactive)
		os.Exit(0)
	}

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

// Метод создания подключения к minio серверу
func makeClient() (*minio.Client, error) {
	minioClient, e := minio.New(*EndPoint+":"+*Port, &minio.Options{
		Creds:           credentials.NewStaticV4(*AccessKeyID, *SecretAccessKey, ""),
		Secure:          *UseSSL,
		TrailingHeaders: true,
		MaxRetries:      10,
	})

	if e != nil {
		return nil, e
	}
	logrus.Info("client created")

	b := utils.ConnectionHealthCheck(minioClient)
	if !b {
		logrus.Error("connection health check failed")
		return nil, errors.New("connection health check failed")
	}
	logrus.Info("connection health check passed")
	return minioClient, nil
}

// Метод чтения кофигурационного файла
// Конфигурационный файл должен быть в формате YAML
// Поиск файла осуществляется в следующем порядке:
// 1. Поиск в ./config.yaml
// 2. Поиск в ./config/config.yaml
// 3. Поиск в ./.config/config.yaml
// 4. Если удается определить домашнюю директорию пользователя то в HOMEDIR/.config/config.yaml
func readcfg() (global.Cfg, error) {
	var cfg global.Cfg
	// Директории поиска конфгурационного файла
	dirs := []string{"./", "./config/", "./.config/"}
	h, e := os.UserHomeDir()
	if e != nil {
		dirs = append(dirs, h+"/.config/")
	}

	for _, dir := range dirs {
		_, e = os.Stat(dir + "config.yaml")
		if e != nil {
			continue
		}
		b, e := os.ReadFile(dir + "config.yaml")
		if e != nil {
			continue
		}

		e = yaml.Unmarshal(b, &cfg)
		if e != nil {
			continue
		}

		return cfg, nil
	}

	return cfg, errors.New("configuration file not found")
}
