package slave

import (
	"context"
	"encoding/json"
	"errors"
	"minioclient/global"
	api "minioclient/runner/api"
	models "minioclient/runner/db"
	globals "minioclient/runner/slave/globals"
	slaveutils "minioclient/runner/slave/utils"
	runnerutils "minioclient/runner/utils"
	"time"

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
		e := migrateTask(task, ctx, me, conn)
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
