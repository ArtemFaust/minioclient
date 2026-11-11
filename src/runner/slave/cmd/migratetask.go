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
	"minioclient/runner/memdb"
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

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/notification"
	"github.com/sirupsen/logrus"
	"github.com/tidwall/buntdb"
	"google.golang.org/grpc"
	"gopkg.in/yaml.v2"
)

// Структура описания событий нотификации
type EventsRecords struct {
	Records []notification.Event `json:"Records"`
}

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
	TaskContext       context.Context          // Контекст управления заданием
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
	workeruuid, _, _, e := slaveutils.GenUuid() // UUID воркера
	if e != nil {
		return fmt.Errorf("[PreMigrate] Ошибка генерации uuid worker: %s", e.Error())
	}

	// Если запучен в контейнере то имя или IP хостового узла должна быть переданна как ENV
	// Если env HOST_RUNNER не определенна то мы запученны не в контейнере и пытаемся извлеч наш hostname
	// Если не удалось получить то в таком случае это не позволяет запустить выполнение миграции так как нотификацию сделать не получится
	var host string
	host = os.Getenv("HOST_RUNNER")
	if host == "" {
		host, e = os.Hostname()
		if e != nil || host == "" {
			return fmt.Errorf("[PreMigrate] Ошибка лпределения hostname: %s", e.Error())
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
		return fmt.Errorf("[PreMigrate] Ошибка создания топика: %s", e.Error())
	}

	// Создаем объект инфорации о бакете
	bi := bucketoperations.BucketInfo{
		Name: &m.TaskWorker.FromBucket,
	}
	e = bi.Make(m.Client)
	if e != nil {
		return fmt.Errorf("[PreMigrate] Ошибка получения информации о бакете источника: %s", e.Error())
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
	addtipicstatus := bi.NotificationConfiguration.AddTopic(notification.Config{
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
	if !addtipicstatus {
		return fmt.Errorf("[PreMigrate] Ошибка создания топика нотификации")
	}
	e = bi.ChangeNotificationConfig(m.Client, bi.NotificationConfiguration)
	if e != nil {
		return fmt.Errorf("[PreMigrate] Ошибка создания нотификации: %s", e.Error())
	}
	return nil
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
				logrus.Warnf("[PostMigrate] Некорректный ARN топика: %s", arn)
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
				logrus.Error("[PostMigrate] Ошибка удаления нтификации")
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
		logrus.Infof("[MasterEmit] Обновление глобального статуса задачи: status = %s", status)
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
			logrus.Errorf("[migrateTask - defer] Ошибка выполнения сброса memdb: %s", e.Error())
		} else {
			logrus.Info("[migrateTask - defer] Сброс memdb выполнен успешно")
		}
		e = properties.MasterEmit("")
		if e != nil {
			logrus.Errorf("[migrateTask - defer] Ошибка финализации отчета выполнения: %s", e.Error())
		}
		properties.TaskContextCancel()
	}()

	// Обновление статуса задачи методом UpdateMigrateStatus
	e := properties.MasterEmit("in-progress")
	if e != nil {
		globals.TaskStatus = "error"
		logrus.Error("[migrateTask] Ошибка выполнения запроса на обновление статуса задачи: ", e.Error())
		return errors.New("Ошибка выполнения запроса на обновление статуса задачи (задача не будет выполненна): " + e.Error())
	}

	logrus.Infof("[migrateTask] Количество батчей/потоков = %d", properties.Threads)

	e = properties.PreMigrate()
	if e != nil {
		logrus.Errorf("[migrateTask] Ошибка выполнения premigrate задач: %s", e.Error())
		e := properties.MasterEmit("error")
		if e != nil {
			globals.TaskStatus = "error"
			logrus.Error("[migrateTask] Ошибка выполнения запроса на обновление статуса задачи: ", e.Error())
			return errors.New("Ошибка выполнения запроса на обновление статуса задачи (задача не будет выполненна): " + e.Error())
		}
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
		report:
			for {
				select {
				case <-properties.TaskContext.Done():
					logrus.Info("[migrateTask - thread] Контекст выполнения задачи завершен")
					break report
				case <-properties.GlobalContext.Done():
					logrus.Info("[migrateTask - thread] Глобальный контекст выполнения задачи завершен")
					break report
				case <-tick:
					// Тут реализация отправки статистики мастеру
					e := properties.MasterEmit("")
					if e != nil {
						logrus.Errorf("[migrateTask - thread] Ошибка передачи отчета миграции: %s", e.Error())
					} else {
						logrus.Info("[migrateTask - thread] Отправка отчета выполнения миграции - успешно")
					}
				}
			}
		}(wg)

		// Запуск операции миграции
		e := objectoperations.MigrateObjects(properties.Client, properties.TaskWorker.FromBucket,
			properties.TaskWorker.Prefix, properties.TaskWorker.To.Name,
			&properties.TaskWorker.To.UseSSL, properties.Threads, properties.TaskWorker.ToBucket,
			properties.InteractiveFlag, properties.DebugFlag, properties.TaskContext,
			taskcancel, nil, &properties.TaskWorker.To.Region, properties.ServerFlag, properties.Reporter,
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

	// Применяем накопленные изменения в бакете источнике к бакету назначения
	// Данная операция не является фатальной если произвойдет ошибка доливки изменений
	// это не должно блокировать или сообщать как об фактической ошибки миграции
	// так как не является задачей миграции а окончательным действием над данными
	e = applyChanges(properties)
	if e != nil {
		logrus.Errorf("[migrateTask] Ошибка применения накопленных исзменений: %s", e.Error())
	} else {
		logrus.Info("[migrateTask] Применений накопленных изменений выполненно.")
	}

	// Выполнение операции постмиграции
	e = properties.PostMigrate()
	if e != nil {
		logrus.Warnf("[migrateTask] Не удалось выполнить операции постмиграции - топик и нотифкация сохранены Ошибка: %s", e.Error())
	} else {
		logrus.Info("[migrateTask] Выполненна очистка конфигурации топика и нотификации бакета источника")
	}

	return nil
}

// Метод применения изменений накопленных за время миграции бакета
// Продюсер
func applyChanges(properties *MigrateProps) error {
	// Создаем каналы отправки событий
	putevents := make(chan notification.Event, 1000) // Канал отправки событий переливки данных
	delevents := make(chan notification.Event, 1000) // Канал отправки события удаления объектов

	// Запуск обработчиков событий нотификации
	properties.WaitGoup.Add(2)
	go applyChangesPut(properties, putevents)
	go applyChangesDelete(properties, delevents)

	// Вычитываем данные из E-MEM-DB
	e := MDB.DB.View(func(tx *buntdb.Tx) error {
		e := tx.Ascend("", func(key, value string) bool {
			// Выполняем декомпрессию данных нотификации
			decompresed, e := memdb.Decompress([]byte(value))
			if e != nil {
				logrus.Warnf("[applyChanges] Ошибка выполнения декомпресии данных нотификации: %s - событие будет пропущенно", e.Error())
				return true // Продолжение итерации по БД
			}

			// Выполняем дессерилизацию нотифкации - проверяем ее корректность
			// на соответсвие структуре данных нотифкации
			var events EventsRecords
			e = json.Unmarshal(decompresed, &events)
			if e != nil {
				logrus.Warnf("[applyChanges] Ошибка десерилизации события нотификации: %s - событие будет пропущенно", e.Error())
				return true // Продолжение итерации по БД
			}

			// Обрабатываем полученные события нотификации в разрезе 1 ключа E-MDB
			// События DELETE помечаются в канал
			for _, event := range events.Records {
				operation := event.EventName   // Тип операции над объектом (ObjectCreated:Put || ObjectRemoved:Delete)
				bucket := event.S3.Bucket.Name // KEY объекта

				// На всякий случай проверяем что полученное событие относится к бакету источника операции миграции
				if bucket != properties.TaskWorker.FromBucket {
					logrus.Warnf("[applyChanges] Обрабатываемое событие не относится у бакету источника миграции. Истоник миграции: %s Бакет нотификации: %s",
						properties.TaskWorker.FromBucket, bucket)
					continue
				}

				// Тут просматривается потенциальная проблема взаимных блокировок
				// Если один из каналов забьется это остановит цикл и тем самым не позволит обрабатывать данные предназначенные
				// второму каналу надо исправить

				// Обработка события PUT
				// Отправляем событие в канал сопроцесса доливки данных
				if operation == "ObjectCreated:Put" {
					putevents <- event // Так как каналы буферизированны то блокируется только если канал переполнен
					continue
				}

				// Обработка события DELETE
				// Отправляем ключ объекта события удаления в канал сопроцесса удаления объектов
				if operation == "ObjectRemoved:Delete" {
					delevents <- event // Так как каналы буферизированны то блокируется только если канал переполнен
					continue
				}

				logrus.Warnf("[applyChanges] Не удалось определить метод выполняемый над объектом. Пропуск объекта %s", key)
			}

			return true // Продолжение итерации по БД
		})
		return e // выход из метода обработки натификаций
	})

	// Закрываем каналы передачи данны в сопроцессы обработки событий нотификаций
	// после окончания обработки нтификаций. После закрытия канала сопроцессы завершат свою работу как
	// только дочитают все данные из буфера
	close(putevents)
	close(delevents)
	properties.WaitGoup.Wait()
	return e
}

// Метод обработки операции доливки данных из бакета источника в бакет миграции
// Консаммер
func applyChangesPut(properties *MigrateProps, putevents chan notification.Event) error {

	const batchsize = 1000                             // Размер 1 батча для сопроцесса
	const mutipartThreshold = 33554432                 // Пороговый размер объекта для срабатывания мультипарта
	signalch := make(chan bool, runtime.NumCPU()*2)    // Управляющий канал ограниченный кол-вом CPU*2
	events := make([]notification.Event, 0, batchsize) // Буфер ключей
	var localwg sync.WaitGroup                         // Локальная группа синхронизации горутин
	defer localwg.Wait()                               // Ожидаем окончания работы всех горутин
	defer properties.WaitGoup.Done()

	// Создание клиента подключения к кластеру назначения миграции и применения накопленных изменений
	toclient := global.GlobalClient{}
	tclient, toclientconnectionconfig, e := utils.InitClient(&properties.TaskWorker.To.Name, nil, nil, nil, &properties.TaskWorker.To.UseSSL, false,
		&properties.TaskWorker.To.Region, true)
	if e != nil {
		return e
	}
	toclient.MinioClient = tclient
	toclient.ClientCfg = toclientconnectionconfig

	// Экземпляр сопроцеса обработки объектов
	worker := func(localwg *sync.WaitGroup, batch []notification.Event, properties *MigrateProps, toclient *global.GlobalClient) {
		defer func() { <-signalch; localwg.Done() }() // Высвобюождаем слот и сообщаем об окончании сопроцесса
		// Обрабатываем все объекты которые пришли как событие нотификации и переданны в сопроцесс
		for _, event := range batch {
			select {
			case <-properties.GlobalContext.Done():
				return
			default:
				properties.Reporter.Difference.Store(properties.Reporter.Difference.Load() - 1) // Уменьшаем кол-во разностных объектов на 1
				func() {
					// На всякий случай проверяем что ключ не пустая строка
					if event.S3.Object.Key == "" {
						logrus.Warnf("[applyChangesPut] пустой ключ в событии, пропуск")
						return
					}

					// Считываем объект из бакета источника
					sourceobj, e := properties.Client.MinioClient.GetObject(properties.GlobalContext, properties.TaskWorker.FromBucket,
						event.S3.Object.Key, minio.GetObjectOptions{
							VersionID: "", // Версионирование не поддерживается мы работаем только с актуальной версией объекта в бакете
							Checksum:  true,
						})
					if e != nil {
						logrus.Errorf("[applyChangesPut] Ошибка получения объекта %s из бакета источника: %s", event.S3.Object.Key, e.Error())
						properties.Reporter.ErrorObjects.Add(1)
						return
					}
					defer sourceobj.Close() // Закрываем исходный объект

					// Опции загрузки объекта для бакета назначения
					tooptions := minio.PutObjectOptions{
						ContentType:           "application/octet-stream",
						NumThreads:            uint(runtime.NumCPU()),                   // Кол-во поток загрузки объекта в бакет по кол-ву CPU
						ConcurrentStreamParts: toclient.ClientCfg.ConcurrentStreamParts, // Конкурентная загрузка
						DisableMultipart:      toclient.ClientCfg.DisableMultipart,
						Checksum: func() minio.ChecksumType {
							// Если в конфиге маультипарт отключен то хешсумму не читаем
							if toclient.ClientCfg.DisableMultipart {
								return minio.ChecksumNone
							}
							// Если не указан тип хеша то используем по умолчанию ChecksumCRC64NVME
							if toclient.ClientCfg.MultipartChecksSum == "" {
								return minio.ChecksumCRC64NVME
							}
							// ChecksumCRC32|ChecksumCRC32C|ChecksumCRC64NVME|ChecksumMD5|ChecksumSHA1|ChecksumSHA256|ChecksumSHA512|ChecksumXXHASH3|ChecksumXXHASH64
							switch toclient.ClientCfg.MultipartChecksSum {
							case "ChecksumCRC32":
								return minio.ChecksumCRC32
							case "ChecksumCRC32C":
								return minio.ChecksumCRC32C
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
							if toclient.ClientCfg.DisableMultipart {
								return minio.ChecksumNone
							}
							// Если не указан тип хеша то используем по умолчанию ChecksumCRC64NVME
							if toclient.ClientCfg.MultipartChecksSum == "" {
								return minio.ChecksumCRC64NVME
							}
							// ChecksumCRC32|ChecksumCRC32C|ChecksumCRC64NVME|ChecksumMD5|ChecksumSHA1|ChecksumSHA256|ChecksumSHA512|ChecksumXXHASH3|ChecksumXXHASH64
							switch toclient.ClientCfg.MultipartChecksSum {
							case "ChecksumCRC32":
								return minio.ChecksumCRC32
							case "ChecksumCRC32C":
								return minio.ChecksumCRC32C
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
						SendContentMd5: toclient.ClientCfg.SendMd5CheckSum,
					}

					// Пулучаем стату объекта
					stat, e := sourceobj.Stat()
					if e != nil {
						logrus.Errorf("[applyChangesPut] Ошибка получения stat объекта из бакета источника: %s", e.Error())
						properties.Reporter.ErrorObjects.Add(1)
						return
					}

					// Если объект маленький (менее 15 МиБ) то принудительно отключаем мультипарт загрузку для него
					// а так же отключаем вычисление хешей
					if stat.Size < mutipartThreshold {
						tooptions.DisableMultipart = true
						tooptions.Checksum = minio.ChecksumNone
						tooptions.AutoChecksum = minio.ChecksumNone
					}

					// Переливаем объект в бакет назначения
					_, e = toclient.MinioClient.PutObject(properties.GlobalContext, properties.TaskWorker.ToBucket,
						event.S3.Object.Key, sourceobj, stat.Size, tooptions)
					if e != nil {
						logrus.Errorf("[applyChangesPut] Ошибка выполнения операции переливки объекта %s из бакета источника в бакет назначения: %s",
							event.S3.Object.Key, e.Error())
						properties.Reporter.ErrorObjects.Add(1)
						return
					}
				}()
			}
		}
	}

	// Обрабатываем все оъекты полученные из канала
	for event := range putevents {
		events = append(events, event) // Добавляем событие в буфер
		if len(events) == batchsize {
			signalch <- true                                   // Занимаем слот
			localwg.Add(1)                                     // Добавляем горутину в группу ожидания
			go worker(&localwg, events, properties, &toclient) // Запускаем сопроцесс обработчика переливки объектов
			events = make([]notification.Event, 0, batchsize)  // Сбрасываем буфер после копирования в сопроцесс
		}
	}

	// Дообрабатываем остатки
	if len(events) > 0 {
		signalch <- true                                   // Занимаем слот
		localwg.Add(1)                                     // Добавляем горутину в группу ожидания
		go worker(&localwg, events, properties, &toclient) // Запускаем сопроцесс
	}

	return nil
}

// Метод применения операции удаления объектов в бакете назначения миграции
// на основе изменений в бакете источника
// Консаммер
func applyChangesDelete(properties *MigrateProps, delevents chan notification.Event) error {
	defer properties.WaitGoup.Done() // Сообщаяем о завершении горутины

	// Группа ожидания завершения горутин в рамках данного метода
	var localwg sync.WaitGroup

	// Канал передачи информации об объекте в сопроцесс удаления объектов
	// Канал закрывается в управляющей горутине получения методанных объектов
	delobjectinfo := make(chan minio.ObjectInfo, 1000)

	// Создание клиента подключения к кластеру назначения миграции и применения накопленных изменений
	dstclient, _, e := utils.InitClient(&properties.TaskWorker.To.Name, &properties.TaskWorker.To.Port,
		&properties.TaskWorker.To.Acesskey, &properties.TaskWorker.To.Secretkey, &properties.TaskWorker.To.UseSSL,
		false, &properties.TaskWorker.To.Region, true)
	if e != nil {
		return e
	}

	// Горутина получения методанных объекта из бакета кластера назначения миграции
	localwg.Add(1)
	go func(wg *sync.WaitGroup, delobjectinfo chan minio.ObjectInfo, client *minio.Client, delevents chan notification.Event) {
		defer wg.Done()
		// Пока поступают ключи в канал объектов на удаление
		// запрашиваем методанные объекта в бакете назначения миграции
		// если методанные не полученны выводим сообщение об ошибке иначе передаем методанные в канал непосредственного обработчика удаления объектов
		for event := range delevents {
			properties.Reporter.Difference.Store(properties.Reporter.Difference.Load() - 1) // Уменьшаем кол-во разностных объектов на 1

			objectinfo, e := dstclient.StatObject(properties.GlobalContext, properties.TaskWorker.ToBucket, event.S3.Object.Key, minio.StatObjectOptions{
				VersionID: event.S3.Object.VersionID,
			})
			if e != nil {
				logrus.Errorf("[applyChangesDelete] Ошибкам получения методанных объекта %s в бакете %s", event.S3.Object.Key, properties.TaskWorker.ToBucket)
				continue
			}

			// Если методанные получены то отправляем мету в канал удаления объектов
			delobjectinfo <- objectinfo
		}
		// После окончания получения методанных объектов
		// закрываем канал отправки объектов на удаление
		close(delobjectinfo)
	}(&localwg, delobjectinfo, dstclient, delevents)

	// Горутина удаления объектов
	localwg.Add(1)
	go func(wg *sync.WaitGroup, delobjectinfo chan minio.ObjectInfo, client *minio.Client) {
		defer wg.Done()
		// Запуск процедуры массовго удаления объектов
		// удаление объектов будет происходить до тех пор пока объекты есть в канале и канал не закрыт
		errorchan := dstclient.RemoveObjects(properties.GlobalContext,
			properties.TaskWorker.ToBucket, delobjectinfo, minio.RemoveObjectsOptions{})
		// Считываем ошибки из канала до тех пор пока канал жив
		for e := range errorchan {
			logrus.Errorf("Ошибка выполнения операции удаления объектов: %s", e.Error())
		}
	}(&localwg, delobjectinfo, dstclient)

	// Ожидаем окончания работы горутин
	localwg.Wait()

	return nil
}
