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

// Структура описывающая общие свойства задачи миграции
type MigrateProps struct {
	WorkerUuid        string                   // UUID ворекра исполнителя
	Reporter          *objectoperations.Report // Глобальный экземпляр reporter
	GlobalContext     context.Context          // Глобальный контекст выполнения
	GrpcClientConn    *grpc.ClientConn         // Глобальный экземпляр подключения к мастеру
	Me                string                   // Endpoint подключения к мастеру
	RawTask           *api.GetTaskResponse     // Сырая задача полученная от мастера
	TaskWorker        models.WorkerMigrateTask // Экземпляр задачи
	Client            *global.GlobalClient     // Глобальная конфигурация клиента
	WaitGoup          *sync.WaitGroup          // Группа ожидания синхронизация горутин
	TaskContext       context.Context          // Контекст управления задаей
	TaskContextCancel context.CancelFunc       // Функция отмены задачи
	InteractiveFlag   bool                     // Флаг интерактивности
	ServerFlag        bool                     // Флаг серверного запуска
	DebugFlag         bool                     // Флаг режима вывода сообщений
	Threads           int                      // Кол-во поток выполнения (деление на батчи)
}

// Метод инициализации экземпляра свойств миграции
func (m *MigrateProps) Init(ctx context.Context, conn *grpc.ClientConn, me string, rawtask *api.GetTaskResponse) error {
	var e error
	m.WorkerUuid, _, _, e = slaveutils.GenUuid()
	if e != nil {
		return e
	}
	m.RawTask = rawtask
	m.GlobalContext = ctx
	m.GrpcClientConn = conn
	m.Me = me
	m.TaskWorker, e = func() (models.WorkerMigrateTask, error) {
		var taskWorker models.WorkerMigrateTask
		b, e := runnerutils.Decoder(string(m.RawTask.Task), m.WorkerUuid)
		if e != nil {
			return taskWorker, e
		}
		if e := json.Unmarshal(b, &taskWorker); e != nil {
			// Ошибка десериализации задачи
			globals.TaskStatus = "error"
			return taskWorker, e
		}
		return taskWorker, nil
	}()
	m.Client.MinioClient, m.Client.ClientCfg, e = utils.InitClient(
		&m.TaskWorker.From.Endpoints[0], &m.TaskWorker.From.Port, &m.TaskWorker.From.Acesskey, &m.TaskWorker.From.Secretkey, &m.TaskWorker.From.UseSSL,
		false, &m.TaskWorker.From.Region, true,
	)
	if e != nil {
		return e
	}

	m.TaskContext, m.TaskContextCancel = context.WithCancel(m.GlobalContext)
	m.InteractiveFlag = false        // флаг что не интерактивный
	m.ServerFlag = true              // флаг что запукчен серверный режим
	m.DebugFlag = true               // флаг включения debug
	m.Threads = runtime.NumCPU() * 2 // кол-во потоков выполнения вдвое больше кол-ва CPU
	return nil
}

// Метод структуры для выполнения premigrate операций
func (m *MigrateProps) PreMigrate() error {
	// Ждем 30 секунд - задержка перед стартом на всякий случай
	time.Sleep(30 * time.Second)

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

	topic := topicoperation.TopicConfig{
		TopicName:         fmt.Sprintf("%s-%s-%s", workeruuid, m.TaskWorker.FromBucket, strconv.Itoa(int(globals.CurrentTaskId.Load()))), // Имя топика (сотосит из workeruuid-srcbucketname-taskid)
		PushEndpoint:      "http://" + host + ":" + port,                                                                                 // Endpoint подключения к данному воркеру
		UseSsl:            false,                                                                                                         // Наш воркер работает на HTTP без SSl
		VerifySsl:         false,                                                                                                         // Мы говорим что даже ели вдруг при подключении окажется что там ssl то не проверять
		HttpAuthUser:      "",                                                                                                            // В нашем воркере нет аутентификации (пока что)
		HttpAuthPassword:  "",                                                                                                            // В нашем воркере нет аутентификации (пока что)
		CloudeventsFormat: true,
	}
	// Создаем новый топик к которому будем привязывать нотификацию
	arn, e := topicoperation.CreateOrUpdateTopic(m.GlobalContext, m.Client.MinioClient, topic)
	if e != nil {
		return e
	}

	// Создаем объект инфорации о бакете
	bi := bucketoperations.BucketInfo{
		Name: &m.TaskWorker.FromBucket,
	}
	e = bi.Make(m.Client)
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
						Name:  "prefix",            // Указываем, что фильтруем по префиксу
						Value: m.TaskWorker.Prefix, // Имя папки в бакете (обязательно со слэшем на конце)
					},
				},
			},
		},
	})

	return bi.ChangeNotificationConfig(m.Client, bi.NotificationConfiguration)
}

// Метод структуры для выполнения postmigrate операций
func (m *MigrateProps) PostMigrate() error {
	// Создаем объект инфорации о бакете
	bi := bucketoperations.BucketInfo{
		Name: &m.TaskWorker.FromBucket,
	}
	e := bi.Make(m.Client)
	if e != nil {
		return e
	}
	// Находим наш топик и выполняем его удаление
	workeruuid, _, _, e := slaveutils.GenUuid() // UUID воркера
	if e != nil {
		return e
	}
	topiclist, e := topicoperation.GetTopicsList(m.GlobalContext, m.Client.MinioClient)
	if e != nil {
		return e
	}
	// Проходим по массиву всех ARN сопоставляем и если найденно то удаляем
	for _, arn := range topiclist {
		if strings.Contains(arn, fmt.Sprintf("%s-%s-%s", workeruuid, m.TaskWorker.FromBucket, strconv.Itoa(int(globals.CurrentTaskId.Load())))) {
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
			e = bi.ChangeNotificationConfig(m.Client, bi.NotificationConfiguration)
			if e != nil {
				logrus.Error("Ошибка удаления ")
				return e
			}
			_, e := topicoperation.RemoveTopic(m.GlobalContext, m.Client.MinioClient, arn)
			if e != nil {
				return e
			}
			return nil
		}
	}

	return nil
}

// Метод структуры отправки обновленного состояния мастеру
func (m *MigrateProps) MasterEmit(status string) error {
	if status != "" {
		globals.TaskStatus = status
	}

	keycount, _ := MDB.GetDBKeyCount()
	m.Reporter.Difference.Store(uint64(keycount))
	return UpdateMigrateStatus(m.Me, m.GlobalContext, m.GrpcClientConn, &api.MigrateReport{
		TotalObjects: m.Reporter.TotalObjects.Load(),
		ErrorObjects: m.Reporter.ErrorObjects.Load(),
		DoneObjects:  m.Reporter.DoneObjects.Load(),
		Difference:   m.Reporter.Difference.Load(),
	})
}

// Метод структуры создания конфигурационного файла
func (m *MigrateProps) CreateConfig() error {
	config := global.Cfg{}
	connectionto := global.Connection{
		Name:                  m.TaskWorker.To.Name,
		Endpoints:             m.TaskWorker.To.Endpoints,
		Port:                  m.TaskWorker.To.Port,
		Region:                m.TaskWorker.To.Region,
		UseSSL:                m.TaskWorker.To.UseSSL,
		MultipartChecksSum:    m.TaskWorker.To.MultipartChecksSum,
		DisableMultipart:      m.TaskWorker.To.DisableMultipart,
		SendMd5CheckSum:       m.TaskWorker.To.SendMd5CheckSum,
		ConcurrentStreamParts: m.TaskWorker.To.ConcurrentStreamParts,
		Acesskey:              m.TaskWorker.To.Acesskey,
		Secretkey:             m.TaskWorker.To.Secretkey,
	}
	connectionfrom := global.Connection{
		Name:                  m.TaskWorker.From.Name,
		Endpoints:             m.TaskWorker.From.Endpoints,
		Port:                  m.TaskWorker.From.Port,
		Region:                m.TaskWorker.From.Region,
		UseSSL:                m.TaskWorker.From.UseSSL,
		MultipartChecksSum:    m.TaskWorker.From.MultipartChecksSum,
		DisableMultipart:      m.TaskWorker.From.DisableMultipart,
		SendMd5CheckSum:       m.TaskWorker.From.SendMd5CheckSum,
		ConcurrentStreamParts: m.TaskWorker.From.ConcurrentStreamParts,
		Acesskey:              m.TaskWorker.From.Acesskey,
		Secretkey:             m.TaskWorker.From.Secretkey,
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

// Метод выполнения задачи миграции бакета
func migrateTask(properties *MigrateProps) error {
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
		e = properties.MasterEmit("")
		if e != nil {
			logrus.Errorf("Ошибка финализации отчета выполнения: %s", e.Error())
		}
		properties.TaskContextCancel()
	}()

	// Обновление статуса задачи методом UpdateMigrateStatus
	e := properties.MasterEmit("in-progress")
	if e != nil {
		globals.TaskStatus = "error"
		logrus.Error("Ошибка выполнения запроса на обновление статуса задачи: ", e.Error())
		return errors.New("Ошибка выполнения запроса на обновление статуса задачи (задача не будет выполненна): " + e.Error())
	}

	logrus.Infof("Количество батчей/потоков = %d", properties.Threads)

	e = properties.PreMigrate()
	if e != nil {
		return e
	}

	// Запуск процесса миграции выполняется фоново от основного процесса
	// с блокировкой окончания
	// Поток выполнения миграции
	properties.WaitGoup.Add(1)
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
				case <-properties.TaskContext.Done():
					return
				case <-properties.GlobalContext.Done():
					return
				case <-tick:
					// Тут реализация отправки статистики мастеру
					e := properties.MasterEmit("")
					if e != nil {
						logrus.Errorf("Ошибка передачи отчета миграции: %s", e.Error())
					} else {
						logrus.Info("Отправка отчета выполнения миграции - успешно")
					}
				}
			}
		}(wg)

		// Запуск операции миграции
		e := objectoperations.MigrateObjects(properties.Client, properties.TaskWorker.FromBucket, properties.TaskWorker.Prefix, properties.TaskWorker.To.Name,
			&properties.TaskWorker.To.UseSSL, properties.Threads, properties.TaskWorker.ToBucket, properties.InteractiveFlag, properties.DebugFlag, properties.TaskContext, taskcancel, nil, &properties.TaskWorker.To.Region, properties.ServerFlag, properties.Reporter,
		)
		if e != nil {
			globals.TaskStatus = "error"
		} else {
			globals.TaskStatus = "done"
		}
		properties.Reporter.PrintStats()
	}(properties.WaitGoup, properties.TaskContextCancel)

	// Поток отслеживания состояния задачи - сигнала отмены выполнения
	// поток отслеживания завершается если основная задача миграции закончилась или если пришол сигнал отмены задачи
	properties.WaitGoup.Add(1)
	go func(wg *sync.WaitGroup, taskcancel context.CancelFunc) {
		defer wg.Done()
		ticker := time.Tick(1 * time.Second)
		for {
			select {
			case <-properties.TaskContext.Done():
				return
			case <-ticker:
				// Тут запрос состояния отмены задачи
				// отмена контекста выполнения горутины миграции по полученному состоянию
				// taskcancel()
				// return
			}
		}
	}(properties.WaitGoup, properties.TaskContextCancel)
	properties.WaitGoup.Wait()

	// Выполнение операции постмиграции
	e = properties.PostMigrate()
	if e != nil {
		logrus.Warnf("Не удалось выполнить операции постмиграции - топик и нотифкация сохранены Ошибка: %s", e.Error())
	} else {
		logrus.Info("Выполненна очистка конфигурации топика и нотификации бакета источника")
	}

	return nil
}

// Метод применения изменений накопленных за время миграции бакета
func applyChanges(properties *MigrateProps) error {
	return nil
}
