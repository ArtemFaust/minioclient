package slave

import (
	"context"
	"encoding/json"
	"errors"
	"minioclient/global"
	objectoperations "minioclient/object_operations"
	api "minioclient/runner/api"
	models "minioclient/runner/db"
	globals "minioclient/runner/slave/globals"
	slaveutils "minioclient/runner/slave/utils"
	runnerutils "minioclient/runner/utils"
	utils "minioclient/utils"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/ghodss/yaml"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

// Глобальная переменная клиента
var Client global.GlobalClient

// метод инициализации запуска задачи
// определяем тип задачи
// сообщаем мастеру что задача взята выполняется - глобально так же ставим что задача в работе
// выполняем задачу - переодически шлем мастеру статистику по задаче
// сообщаем мастеру результат выполнения задачи
// глобально ставим параметры что воркер освободился
func RunTask(task *api.GetTaskResponse, ctx context.Context, me string, conn *grpc.ClientConn) error {
	if task == nil {
		return errors.New("Ошибка task is nil")
	}

	// Глобально выставляем статусы
	globals.CurrentTaskId.Store(task.Id)
	globals.Status.Store(true)
	globals.TaskStatus = "Prepaire task for launche"
	time.Sleep(30 * time.Second)

	// Отложенноя установка глобальных перемпенных
	defer globals.Status.Store(false)

	// Определение типа задачи
	taskname, e := getTaskType(task.Task)
	if e != nil {
		return e
	}

	// В зависимости от типа задачи инциируем запуск ее выполнения
	switch {
	case taskname == "Migrate bucket":
		e := runMigrateTask(task, ctx, me, conn)
		if e != nil {
			logrus.Error("Ошибка выполнения задачи миграция бакета: ", e.Error())
			return e
		}
	default:
		logrus.Warn("Не удалось определить тип задачи")
		globals.TaskStatus = "error" // Ставим статус задачи
		// Обновление статуса задачи методом UpdateMigrateStatus
		return errors.New("Не удалось определить тип задачи")
	}

	return nil
}

// Метод определения типа задачи
func getTaskType(raw []byte) (string, error) {
	// выполняем декодирование задачи
	uuid, _, _, e := slaveutils.GenUuid()
	if e != nil {
		return "", e
	}
	b, e := runnerutils.Decoder(string(raw), uuid)
	if e != nil {
		return "", e
	}

	// Задача миграции бакета
	var taskWorker models.WorkerMigrateTask
	if e := json.Unmarshal(b, &taskWorker); e == nil {
		// Успешно десериализовано в WorkerMigrateTask
		return taskWorker.TaskName, nil
	}

	return "", errors.New("Ошибка определения типа задачи")

}

// Метод выполнения задачи миграции бакета
func runMigrateTask(task *api.GetTaskResponse, ctx context.Context, me string, conn *grpc.ClientConn) error {
	defer UpdateMigrateStatus(me, ctx, conn)

	// Задача миграции бакета
	// Обновление статуса задачи методом UpdateMigrateStatus
	globals.TaskStatus = "in-progress"
	e := UpdateMigrateStatus(me, ctx, conn)
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
		reporter := objectoperations.Report{}
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
					reporter.PrintStats()
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

	e = UpdateMigrateStatus(me, ctx, conn)
	if e != nil {
		logrus.Error("Ошибка выполнения запроса на обновление статуса задачи: ", e.Error())
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
