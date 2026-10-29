package cmd

// Пакет описания методов RPC сервиса master

import (
	"context"
	"encoding/json"
	"fmt"
	"minioclient/global"
	api "minioclient/runner/api"
	"minioclient/runner/db"
	"minioclient/runner/utils"
	"net"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/peer"
	"gorm.io/gorm"
)

// Струткура реализует методы GRPC сервиса Master
type Master struct {
	Db    *gorm.DB
	Dbcfg *global.DBConfig
}

// Метод установления соединения slave с master
// Если worker проходит все проверки то
// если его нет в БД он добавляется
// иначе обновляются данные воркера и статусы
// если воркер выполняет какие либо задачи то статусы задач обновляются на переданные воркером статусы
func (rpc *Master) ClientHello(ctx context.Context, req *api.ClientHelloRequest) (*api.ClientHelloResponse, error) {
	response := &api.ClientHelloResponse{}
	// Проверка входных данных

	// Валилдируем IP

	// Получаем IP из контекста
	if p, ok := peer.FromContext(ctx); ok {
		req.IpAddress = p.Addr.String()
	}

	/*
		if _, e := netip.ParseAddr(req.IpAddress); e != nil {
			response.Status = "Denny"
			response.StatusDetailed = "Error validate IP address: " + e.Error()
			return response, nil
		}
	*/

	// Валидируем MAC
	if _, e := net.ParseMAC(req.MacAddress); e != nil {
		response.Status = "Denny"
		response.StatusDetailed = "Error validate MAC address: " + e.Error()
		return response, nil
	}
	// Валидируем доступность памяти
	if req.MemTotal == 0 {
		response.Status = "Denny"
		response.StatusDetailed = "Total awaliable memory is low"
		return response, nil
	}
	// Валидируем кол-во cpu
	if req.CpuCount == 0 {
		response.Status = "Denny"
		response.StatusDetailed = "Total awaliable cpu is low"
		return response, nil
	}

	// Генерируем UUID из mac + hostname и валидируем с переданным со стороны клиента
	uuid := uuid.NewSHA1(uuid.NameSpaceX500, []byte(req.MacAddress+req.HostName))

	if req.Uuid != uuid.String() {
		response.Status = "Denny"
		response.StatusDetailed = "The provided UUID does not match the one generated on the server side."
		return response, nil
	}

	// Создаем новый объект worker
	worker := db.SlaveWorker{
		Uuid:          uuid.String(),      // UUID воркера
		HostName:      req.HostName,       // HOSTNAME воркера
		MacAddress:    req.MacAddress,     // MAC address воркера
		IpAddress:     req.IpAddress,      // IP address воркера
		OsType:        req.OsType,         // ОС воркера
		MemTotal:      uint(req.MemTotal), // Доступная память воркера
		CpuCount:      req.CpuCount,       // Доступное кол-во cpu
		Online:        true,               // Доступность (так как запрос пришел то он доступен)
		Status:        req.Status,         // Статус занятости
		CurrentTaskId: req.TaskId,         // Чем занят - id задачи
		TaskStatus:    req.TaskStatus,     // Статус выполянемой задачи
	}

	// Если клиент не добавлен в базу - то добавляем
	if !worker.CheckIfExists(rpc.Db) {
		e := worker.AddNewWorker(rpc.Db, rpc.Dbcfg)
		if e != nil {
			response.Status = "Denny"
			response.StatusDetailed = "Error add new worker to database: " + e.Error()
			logrus.Error("Ошибка добавление нового worker UUID: ", worker.Uuid)
			return response, nil
		} else {
			logrus.Info("Добавление нового worker UUID: ", worker.Uuid)
			response.Status = "Allow"
			response.StatusDetailed = "Sucessfule add new worker: UUID=" + uuid.String()
			return response, nil
		}
	} else {
		// Обновляем информацию о worker
		e := worker.Update(rpc.Db)
		if e != nil {
			logrus.Error("Ошибка обновления статуса worker")
		}
	}

	// Если клиент добавлен
	response.Status = "Allow"
	logrus.Info("Успешное подключение worker: ", worker.Uuid)

	return response, nil
}

// Метод запроса задач
func (rpc *Master) GetTasks(ctx context.Context, req *api.GetTaskRequest) (*api.GetTaskResponse, error) {
	logrus.Info("Запрос списка задач от worker: " + req.Uuid)
	response := &api.GetTaskResponse{}
	// Создаем новый объект worker
	worker := db.SlaveWorker{
		Uuid: req.Uuid,
	}

	// Если worker не сучествует то на запрос задачи возвращаем ошибку
	if !worker.CheckIfExists(rpc.Db) {
		logrus.Error("Ошибка валидации worker: ", req.Uuid)
		response.Error = true
		response.AdditionalMessage = "Error validate worker. Worker not registered on master!"
		return response, nil
	}

	// Получаем задачи предназначенные запросившему воркеру
	task := db.Task{
		Uuid: req.Uuid,
	}

	tasks, e := task.GetWorkerNewTaskList(rpc.Db)
	if e != nil {
		logrus.Error("Ошибка получения списка задач для worker: ", req.Uuid)
		response.Error = true
		response.AdditionalMessage = "Error get worker task list: " + e.Error()
		return response, nil
	}

	// Если задач нету
	if len(tasks) == 0 {
		logrus.Info("Список задач пуст для worker: ", req.Uuid)
		response.Error = false
		response.AdditionalMessage = "There are no new tasks for worker"
		return response, nil
	}

	// Находим задачу с наибольшим приоритетом и которая долбше всего ожидает выполнения

	// Инициализируем переменные значениями первой задачи
	p := tasks[0].Priority
	taskindex := 0
	taskCreateAt := tasks[0].CreatedAt

	// находим задачу с наибольшим приоритетом или задачу которая дольше всего ждет
	for i := 1; i < len(tasks); i++ {
		t := tasks[i]
		// Условие 1: Нашли задачу с более высоким приоритетом (число меньше)
		// Условие 2: Приоритеты равны, но текущая задача создана раньше (дольше ждет)
		if t.Priority < p || (t.Priority == p && t.CreatedAt.Before(taskCreateAt)) {
			p = t.Priority
			taskindex = i
			taskCreateAt = t.CreatedAt
		}
	}
	logrus.Info("Найдена задача для worker: ", req.Uuid)

	// Сериализуем данные
	b, e := json.Marshal(tasks[taskindex].WorkerTask)
	if e != nil {
		logrus.Error(fmt.Sprintf("Ошибка сериализации json объекта задачи с ID %s. Ошибка: %s", tasks[taskindex].ID, e.Error()))
		response.Error = true
		response.AdditionalMessage = "Error marchal worker task list: " + e.Error()
		return response, nil
	}

	// Обновляем статус задачи в БД
	tasks[taskindex].Status = "delegated"
	e = tasks[taskindex].ChangeTaskStatus(rpc.Db)
	if e != nil {
		logrus.Error("Ошибка обновления статуса задачи ID: ", tasks[taskindex].ID, " UUID: ", tasks[taskindex].Uuid)
		response.Error = false
		response.AdditionalMessage = "Error update task status on master"
		return response, nil
	}

	logrus.Info("Обновление статуса задачи ID: ", tasks[taskindex].ID, " UUID: ", tasks[taskindex].Uuid, " статус: delegated")

	// шифруем тело задачи
	eb, e := utils.Encoder(string(b), tasks[taskindex].Uuid)
	if e != nil {
		logrus.Error("Ошибка шифрования задачи с ID: ", tasks[taskindex].ID, " UUID: ", tasks[taskindex].Uuid, " ОШИБКА: ", e.Error())
		response.Error = false
		response.AdditionalMessage = "Error encode task status on master: " + e.Error()
		return response, nil
	}

	// Формируем ответ и отправляем задачу воркеру
	response.Id = uint64(tasks[taskindex].ID)     // ID задачи
	response.Uuid = tasks[taskindex].Uuid         // Передаем UUID для проверки на строне клиента что это его задача
	response.Priority = tasks[taskindex].Priority // Передаем приоритете задачи
	response.Status = tasks[taskindex].Status     // Передаем статус задачи - что она делегированна ему для проверки на стороне клиента
	response.Task = eb                            // Передаем саму задачу в шифрованном виде
	response.Error = false
	response.AdditionalMessage = "New delegated task"
	logrus.Info("Отправленна новая задача для worker: ", req.Uuid)

	return response, nil
}

// Метод запроса обновления статуса задачи миграции
func (rpc *Master) UpdateMigrateTaskStatus(ctx context.Context, req *api.UpdateMigrateStatusRequest) (*api.UpdateMigrateStatusResponse, error) {
	logrus.Info("Обновление статуса задачи: ", req.TaskStatus.Id, " от worker ", req.Uuid)
	response := &api.UpdateMigrateStatusResponse{}

	// Обновление статуса задачи таблици tasks
	task := db.Task{
		ID:     uint(req.TaskStatus.Id),
		Uuid:   req.Uuid,
		Status: req.TaskStatus.Status,
	}

	e := task.ChangeTaskStatus(rpc.Db)
	if e != nil {
		logrus.Error("Ошибка обновления статуса задачи: ", task.ID, " воркера: ", task.Uuid)
		response.Uuid = req.Uuid
		response.UpdateStatus = false
		response.AdditionalMessage = "Error update task status: " + e.Error()
		return response, nil
	}

	logrus.Info("Выполненно бновления статуса задачи: ", task.ID, " воркера: ", task.Uuid)

	// Создание или обновление записи report
	report := db.MigrateReport{
		Uuid:         req.Uuid,
		TaskID:       uint(req.TaskStatus.Id),
		TotalObjects: uint(req.MigrateReport.TotalObjects),
		ErrorObjects: uint(req.MigrateReport.ErrorObjects),
		DoneObjects:  uint(req.MigrateReport.DoneObjects),
		Errors:       req.MigrateReport.Errors,
	}

	e = report.CreateOrUpdateMigrateReport(rpc.Db)
	if e != nil {
		logrus.Error("Ошибка обновления report задачи: ", task.ID, " воркера: ", task.Uuid)
		response.Uuid = req.Uuid
		response.UpdateStatus = false
		response.AdditionalMessage = "Error update report: " + e.Error()
		return response, nil
	}

	response.Uuid = req.Uuid
	response.UpdateStatus = true
	response.AdditionalMessage = "Succesfule update task and report status: " + req.TaskStatus.Status
	return response, nil

}
