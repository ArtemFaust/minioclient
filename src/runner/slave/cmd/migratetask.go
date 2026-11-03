package slave

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	bucketoperations "minioclient/bucket_operations"
	"minioclient/global"
	objectoperations "minioclient/object_operations"
	api "minioclient/runner/api"
	models "minioclient/runner/db"
	globals "minioclient/runner/slave/globals"
	slaveutils "minioclient/runner/slave/utils"
	runnerutils "minioclient/runner/utils"
	topicoperation "minioclient/topicoperations"
	utils "minioclient/utils"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ghodss/yaml"
	"github.com/minio/minio-go/v7/pkg/notification"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

// Метод выполнения задачи миграции бакета
func migrateTask(task *api.GetTaskResponse, ctx context.Context, me string, conn *grpc.ClientConn) error {
	reporter := objectoperations.Report{}

	// После окончания выполнения процесса миграции
	// выполняем финальную отправку состояния и выполняем сброс memdb для очистки памяти
	defer func() {
		// Сброс memDB выполняется в конце после методов доливки
		e := MDB.DropDB()
		if e != nil {
			logrus.Errorf("Ошибка выполнения сброса memdb: %s", e.Error())
		} else {
			logrus.Info("Сброс memdb выполнен успешно")
		}
		keycount, _ := MDB.GetDBKeyCount()
		reporter.Difference.Store(uint64(keycount))
		UpdateMigrateStatus(me, ctx, conn, &api.MigrateReport{
			TotalObjects: reporter.TotalObjects.Load(),
			ErrorObjects: reporter.ErrorObjects.Load(),
			DoneObjects:  reporter.DoneObjects.Load(),
			Difference:   reporter.Difference.Load(),
		})
	}()

	// Задача миграции бакета
	// Обновление статуса задачи методом UpdateMigrateStatus
	globals.TaskStatus = "in-progress"
	keycount, _ := MDB.GetDBKeyCount()
	reporter.Difference.Store(uint64(keycount))
	e := UpdateMigrateStatus(me, ctx, conn, &api.MigrateReport{
		TotalObjects: reporter.TotalObjects.Load(),
		ErrorObjects: reporter.ErrorObjects.Load(),
		DoneObjects:  reporter.DoneObjects.Load(),
		Difference:   reporter.Difference.Load(),
	})
	if e != nil {
		globals.TaskStatus = "error"
		logrus.Error("Ошибка выполнения запроса на обновление статуса задачи: ", e.Error())
		return errors.New("Ошибка выполнения запроса на обновление статуса задачи (задача не будет выполненна): " + e.Error())
	}

	// Запуск миграции
	time.Sleep(30 * time.Second) // Ждем 30 секунд - задержка перед стартом на всякий случай

	// Извлекаем задачу
	var taskWorker models.WorkerMigrateTask
	uuid, _, _, e := slaveutils.GenUuid()
	if e != nil {
		return e
	}
	b, e := runnerutils.Decoder(string(task.Task), uuid)
	if e != nil {
		return e
	}
	if e := json.Unmarshal(b, &taskWorker); e != nil {
		// Ошибка десериализации задачи
		globals.TaskStatus = "error"
		return e
	}

	// Создаем конфигурационный фаил - он нужен для создания toclient
	e = writeConfigFile(taskWorker)
	if e != nil {
		globals.TaskStatus = "error"
		return e
	}

	// Контекст выполнения задачи
	// Данный конттекст будет использоваться для прерывания задачи по сигналу
	taskctx, taskcancel := context.WithCancel(context.Background())
	defer taskcancel()

	// Создание клиента источника миграции
	client, connection, e := utils.InitClient(
		&taskWorker.From.Endpoints[0], &taskWorker.From.Port, &taskWorker.From.Acesskey, &taskWorker.From.Secretkey, &taskWorker.From.UseSSL,
		false, &taskWorker.From.Region, true,
	)
	if e != nil {
		globals.TaskStatus = "error"
		return e
	}

	Client.MinioClient = client
	Client.ClientCfg = connection
	interactive := false // флаг что не интерактивный
	serve := true
	debug := true                   // флаг включения debug
	threads := runtime.NumCPU() * 2 // кол-во потоков выполнения вдвое больше кол-ва CPU
	logrus.Infof("Количество батчей/потоков = %d", threads)

	// Выполнение операций перед началом миграции
	// Если не удается выполнить операции pre миграции то это ошибка не позволяючая продолжать
	workeruuid, _, _, e := slaveutils.GenUuid() // UUID воркера
	if e != nil {
		return e
	}

	// Если запучен в контейнере то имя или IP хостового узла должна быть переданна как ENV
	// Если env HOST_RUNNER не определенна то мы запученны не в контейнере и пытаемся извлеч наш hostname
	// Если не удалось получить то в таком случае это не позволяет запустить выполнение миграции так как нотификацию сделать не получится
	var host string
	host = os.Getenv("HOST_RUNNER")
	if host == "" {
		host, e = os.Hostname()
		if e != nil || host == "" {
			return e
		}
	}

	// Если запучен в контейнере то пытаемся звлеч порт сервиса нотификации из ENV
	// Если HTTP_PORT не определен то берем из аргумента
	var port string
	port = os.Getenv("HTTP_PORT")
	if port == "" {
		port = HTTP_PORT
	}

	e = preMigrate(ctx, &Client, topicoperation.TopicConfig{
		TopicName:         fmt.Sprintf("%s-%s-%s", workeruuid, taskWorker.FromBucket, strconv.Itoa(int(globals.CurrentTaskId.Load()))), // Имя топика (сотосит из workeruuid-srcbucketname-taskid)
		PushEndpoint:      "http://" + host + ":" + port,                                                                               // Endpoint подключения к данному воркеру
		UseSsl:            false,                                                                                                       // Наш воркер работает на HTTP без SSl
		VerifySsl:         false,                                                                                                       // Мы говорим что даже ели вдруг при подключении окажется что там ssl то не проверять
		HttpAuthUser:      "",                                                                                                          // В нашем воркере нет аутентификации (пока что)
		HttpAuthPassword:  "",                                                                                                          // В нашем воркере нет аутентификации (пока что)
		CloudeventsFormat: false,                                                                                                       // Нам не итересен данный формат
	}, taskWorker.FromBucket, taskWorker.Prefix)
	if e != nil {
		return e
	}

	// Запуск процесса миграции выполняется фоново от основного процесса
	// с блокировкой окончания
	var wg sync.WaitGroup // Группа ожидания завершения горутин
	// Поток выполнения миграции
	wg.Add(1)
	go func(wg *sync.WaitGroup, taskcancel context.CancelFunc) {
		defer taskcancel() // отменяем контекст по завершению что бы завершилась горутина отслеживания сигнала отмены
		defer wg.Done()
		// Создание внешнего канала получения статистики
		// выполнения миграции
		// запуск потока чтения данных из канала report
		// и передачи данных на мастера
		wg.Add(1)
		go func(wg *sync.WaitGroup) {
			defer wg.Done()
			tick := time.Tick(2 * time.Second)
			for {
				select {
				case <-taskctx.Done():
					return
				case <-ctx.Done():
					return
				case <-tick:
					// Тут реализация отправки статистики мастеру
					keycount, _ := MDB.GetDBKeyCount()
					reporter.Difference.Store(uint64(keycount))
					e = UpdateMigrateStatus(me, ctx, conn, &api.MigrateReport{
						TotalObjects: reporter.TotalObjects.Load(),
						ErrorObjects: reporter.ErrorObjects.Load(),
						DoneObjects:  reporter.DoneObjects.Load(),
						Difference:   reporter.Difference.Load(),
					})
					if e != nil {
						logrus.Errorf("Ошибка передачи отчета миграции: %s", e.Error())
					} else {
						logrus.Info("Отправка отчета выполнения миграции - успешно")
					}
				}
			}
		}(wg)

		// Запуск операции миграции
		e := objectoperations.MigrateObjects(&Client, taskWorker.FromBucket, taskWorker.Prefix, taskWorker.To.Name,
			&taskWorker.To.UseSSL, threads, taskWorker.ToBucket, interactive, debug, taskctx, taskcancel, nil, &taskWorker.To.Region, serve, &reporter,
		)
		if e != nil {
			globals.TaskStatus = "error"
		} else {
			globals.TaskStatus = "done"
		}
		reporter.PrintStats()
		return
	}(&wg, taskcancel)

	// Поток отслеживания состояния задачи - сигнала отмены выполнения
	// поток отслеживания завершается если основная задача миграции закончилась или если пришол сигнал отмены задачи
	wg.Add(1)
	go func(wg *sync.WaitGroup, taskcancel context.CancelFunc) {
		defer wg.Done()
		for {
			select {
			case <-taskctx.Done():
				return
			default:
				// Тут запрос состояния отмены задачи
				time.Sleep(10 * time.Second)
				// отмена контекста выполнения горутины миграции по полученному состоянию
				// taskcancel()
				// return
			}
		}
	}(&wg, taskcancel)
	wg.Wait()

	keycount, _ = MDB.GetDBKeyCount()
	reporter.Difference.Store(uint64(keycount))
	e = UpdateMigrateStatus(me, ctx, conn, &api.MigrateReport{
		TotalObjects: reporter.TotalObjects.Load(),
		ErrorObjects: reporter.ErrorObjects.Load(),
		DoneObjects:  reporter.DoneObjects.Load(),
		Difference:   reporter.Difference.Load(),
	})
	if e != nil {
		logrus.Error("Ошибка выполнения запроса на обновление статуса задачи: ", e.Error())
	}

	// Выполнение операции постмиграции - применения изменений
	// произошедших в бакете за время миграции бакета
	e = postMigrate(ctx, &Client, taskWorker)
	if e != nil {
		logrus.Warnf("Не удалось выполнить операции постмиграции - топик и нотифкация сохранены Ошибка: %s", e.Error())
	} else {
		logrus.Info("Выполненна очистка конфигурации топика и нотификации бакета источника")
	}

	return nil
}

// Метод создания конфигурационного файла на воркере
// необходимого для инициализации подключения к кластеру назначения
func writeConfigFile(taskWorker models.WorkerMigrateTask) error {
	config := global.Cfg{}
	connectionto := global.Connection{
		Name:                  taskWorker.To.Name,
		Endpoints:             taskWorker.To.Endpoints,
		Port:                  taskWorker.To.Port,
		Region:                taskWorker.To.Region,
		UseSSL:                taskWorker.To.UseSSL,
		MultipartChecksSum:    taskWorker.To.MultipartChecksSum,
		DisableMultipart:      taskWorker.To.DisableMultipart,
		SendMd5CheckSum:       taskWorker.To.SendMd5CheckSum,
		ConcurrentStreamParts: taskWorker.To.ConcurrentStreamParts,
		Acesskey:              taskWorker.To.Acesskey,
		Secretkey:             taskWorker.To.Secretkey,
	}
	connectionfrom := global.Connection{
		Name:                  taskWorker.From.Name,
		Endpoints:             taskWorker.From.Endpoints,
		Port:                  taskWorker.From.Port,
		Region:                taskWorker.From.Region,
		UseSSL:                taskWorker.From.UseSSL,
		MultipartChecksSum:    taskWorker.From.MultipartChecksSum,
		DisableMultipart:      taskWorker.From.DisableMultipart,
		SendMd5CheckSum:       taskWorker.From.SendMd5CheckSum,
		ConcurrentStreamParts: taskWorker.From.ConcurrentStreamParts,
		Acesskey:              taskWorker.From.Acesskey,
		Secretkey:             taskWorker.From.Secretkey,
	}
	config.Connections = append(config.Connections, connectionto)
	config.Connections = append(config.Connections, connectionfrom)
	b, e := yaml.Marshal(config)
	if e != nil {
		return e
	}
	e = os.WriteFile("config.yaml", b, 0777)
	if e != nil {
		return e
	}
	return nil
}

// Метод выполнения операций установки правил нотификации передначалом миграции
func preMigrate(ctx context.Context, client *global.GlobalClient, topic topicoperation.TopicConfig, frombn string, prefix string) error {
	// Создаем новый топик к которому будем привязывать нотификацию
	arn, e := topicoperation.CreateOrUpdateTopic(ctx, client.MinioClient, topic)
	if e != nil {
		return e
	}

	// Создаем объект инфорации о бакете
	bi := bucketoperations.BucketInfo{
		Name: &frombn,
	}
	e = bi.Make(client)
	if e != nil {
		return e
	}

	// Парсим собранную строку arn - пример строки arn:aws:sns:ms-group01::workeruuid-bucketname-taskid
	region := strings.Split(arn, ":")[3]   // Регион из ARN
	resource := strings.Split(arn, ":")[5] // workeruuid-bucketname-taskid

	// Добавляем нашу конфигурацию нотификации
	targetArn := notification.Arn{
		Partition: "aws",
		Service:   "sns",
		Region:    region, // Наш регион
		AccountID: "",
		Resource:  resource, // Название нашего топика
	}
	bi.NotificationConfiguration.AddTopic(notification.Config{
		ID:  topic.TopicName, // ID конфигурации нотификации делаем равным имени топика для однозначного связывания
		Arn: targetArn,
		Events: []notification.EventType{
			notification.ObjectCreatedAll, // Мы будем отслеживать все события солздания объектов в бакете
			notification.ObjectRemovedAll, // Мы будем отслеживать все события удаления объектов в бакете
		},
		// Добавляем префикс как фильтр
		Filter: &notification.Filter{
			S3Key: notification.S3Key{
				FilterRules: []notification.FilterRule{
					{
						Name:  "prefix", // Указываем, что фильтруем по префиксу
						Value: prefix,   // Имя папки в бакете (обязательно со слэшем на конце)
					},
				},
			},
		},
	})

	return bi.ChangeNotificationConfig(client, bi.NotificationConfiguration)
}

// Метод выполнения операции удаления топика и нотификации с бакета
// Сначала удаляем конфигурацию нотификации
// Затем удаляем топик
func postMigrate(ctx context.Context, client *global.GlobalClient, taskWorker models.WorkerMigrateTask) error {
	// Создаем объект инфорации о бакете
	bi := bucketoperations.BucketInfo{
		Name: &taskWorker.FromBucket,
	}
	e := bi.Make(client)
	if e != nil {
		return e
	}
	// Находим наш топик и выполняем его удаление
	workeruuid, _, _, e := slaveutils.GenUuid() // UUID воркера
	if e != nil {
		return e
	}
	topiclist, e := topicoperation.GetTopicsList(ctx, client.MinioClient)
	if e != nil {
		return e
	}
	// Проходим по массиву всех ARN сопоставляем и если найденно то удаляем
	for _, arn := range topiclist {
		if strings.Contains(arn, fmt.Sprintf("%s-%s-%s", workeruuid, taskWorker.FromBucket, strconv.Itoa(int(globals.CurrentTaskId.Load())))) {
			if len(strings.Split(arn, ":")) < 6 {
				logrus.Warnf("Некорректный ARN топика: %s", arn)
				continue
			}
			// Парсим собранную строку arn - пример строки arn:aws:sns:ms-group01::workeruuid-bucketname-taskid
			region := strings.Split(arn, ":")[3]   // Регион из ARN
			resource := strings.Split(arn, ":")[5] // workeruuid-bucketname-taskid
			// Удаляем параметры нотификации связанные с нашим ARN
			bi.NotificationConfiguration.RemoveTopicByArn(notification.Arn{
				Partition: "aws",
				Service:   "sns",
				Region:    region, // Наш регион
				AccountID: "",
				Resource:  resource, // Название нашего топика
			})
			e = bi.ChangeNotificationConfig(client, bi.NotificationConfiguration)
			if e != nil {
				logrus.Error("Ошибка удаления ")
				return e
			}
			_, e := topicoperation.RemoveTopic(ctx, client.MinioClient, arn)
			if e != nil {
				return e
			}
			return nil
		}
	}

	return nil
}

// Метод выполнения операции анализа изменений бакета источника
// и применения этих изменений к бакету назначения
func applyingSourceBucketChangesOnDstBucket() {}
