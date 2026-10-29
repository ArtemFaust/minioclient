package db

import (
	"encoding/json"
	"errors"
	"minioclient/global"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// Создание новой базы данных
// Структура описывающая сущность slave_worker
type SlaveWorker struct {
	ID            uint   `gorm:"primaryKey;"`
	Uuid          string `gorm:"not null;type:uuid;uniqueIndex" json:"Uuid"`    // Уникальный генерируемый идентификатор PK
	HostName      string `gorm:"not null;type:varchar(100);" json:"HostName"`   // Hostname slave воркера
	MacAddress    string `gorm:"not null;type:varchar(100);" json:"MacAddress"` // Идентификатор устройства на сервере
	IpAddress     string `gorm:"not null;type:varchar(100);" json:"IpAddress"`  // IP адресс воркера
	OsType        string `gorm:"not null;type:varchar(100);" json:"OsType"`     // Тип операционной системы
	MemTotal      uint   `gorm:"type:bigint;not null" json:"MemTotal"`          // Кол-во доступной ОЗУ
	CpuCount      int32  `gorm:"type:integer;not null" json:"CpuCount"`         // Кол-во доступных CPU
	Task          Task   `gorm:"foreignKey:Uuid;references:Uuid" json:"task"`   // Ссылка на ключ внешней таблици
	Online        bool   `gorm:"not null;type:boolean;" json:"Online"`          // worker online/offline
	Status        bool   `gorm:"not null;type:boolean;" json:"Status"`          // Статус workera (busy/free)
	CurrentTaskId uint64 `gorm:"type:bigint;not null" json:"CurrentTaskId"`     // ID текучей выполняемой задачи
	TaskStatus    string `gorm:"not null;type:varchar(100);" json:"TaskStatus"` // Статус выполняемой задачи done|in-progress|error
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DeletedAt     gorm.DeletedAt
}

// Метод проверки существавания записи в таблице по ее UUID
func (worker *SlaveWorker) CheckIfExists(db *gorm.DB) bool {
	var count int64
	db.Model(&worker).Where("uuid = ?", &worker.Uuid).Count(&count)
	return count > 0
}

// Метод получения списка workers
func (worker *SlaveWorker) ListAllWorkers(db *gorm.DB) ([]SlaveWorker, error) {
	workers := []SlaveWorker{}
	rows, e := db.Model(&worker).Rows()
	if e != nil {
		return workers, e
	}
	defer rows.Close()

	for rows.Next() {
		var entry SlaveWorker
		e := db.ScanRows(rows, &entry)
		if e != nil {
			return workers, e
		}
		workers = append(workers, entry)
	}

	return workers, nil
}

// Метод получения отдельного Worker
func (worker *SlaveWorker) GetWorker(db *gorm.DB) (SlaveWorker, error) {
	w := SlaveWorker{}
	e := db.Model(&worker).Limit(1).Where("uuid == ?", worker.Uuid).Row().Scan(&w)
	if e != nil {
		return w, e
	}
	return w, nil
}

// Метод добавления нового Worker в БД
func (worker *SlaveWorker) AddNewWorker(db *gorm.DB, cfg *global.DBConfig) error {
	if worker.CheckIfExists(db) {
		return nil
	}

	// Проверяем существовала ли такая запись ранее
	// защита от удаления активнорго воркера - если воркер был удален но он доступен и его UUID совпадает
	// то произойдет перерегистрация воркера если это действие разрешенно
	var deletedWorker SlaveWorker
	e := db.Unscoped().Where("uuid = ? AND deleted_at IS NOT NULL", worker.Uuid).First(&deletedWorker).Error
	// запись найденна
	if e == nil {
		logrus.Warnf("WORKER UUID: %s находится в состоянии -удаленн-", worker.Uuid)
		// возвращаем запись
		if cfg.Db.Allow_Rerigister_Workers {
			logrus.Info("Разрешена перерегистрация worker после удаления - выполнение перерегистрации")
			return db.Unscoped().Model(&deletedWorker).Update("deleted_at", nil).Error
		}
		logrus.Warnf("Запрещенна перерегистрация worker после удаления")
		return errors.New("Воркер удален - перерегистрация запреченна")
	}

	if errors.Is(e, gorm.ErrRecordNotFound) {
		logrus.Infof("Регистрация нового worker UUID: %s", worker.Uuid)
	}

	// Создаем транзакцию на добавление новой записи в БД
	// Вызов транзакции
	e = db.Transaction(func(tx *gorm.DB) error {
		// Создание новой записи в БД
		// Если всё успешно, транзакция автоматически зафиксируется
		if e := tx.Model(&worker).Create(&worker).Error; e != nil {
			return e // откат транзакции
		}
		return nil
	})
	return e
}

// Метод удаления worker
func (worker *SlaveWorker) DeleteWorker(db *gorm.DB) error {
	// Создаем транзакцию на обновление записи в БД
	// Првоеряем существует ли запись с таким UUID в БД
	if !worker.CheckIfExists(db) {
		return nil
	}

	// Вызов транзакции
	e := db.Transaction(func(tx *gorm.DB) error {
		// Удаление сузествующей записи в БД
		// Если всё успешно, транзакция автоматически зафиксируется
		if err := tx.Where("uuid = ?", worker.Uuid).Delete(&SlaveWorker{}).Error; err != nil {
			return err // откат транзакции
		}
		return nil
	})
	return e
}

// Метод обновления информации о worker
func (worker *SlaveWorker) Update(db *gorm.DB) error {
	// Создаем транзакцию на добавление новой записи в БД
	// Вызов транзакции
	e := db.Transaction(func(tx *gorm.DB) error {
		// Создание новой записи в БД
		// Если всё успешно, транзакция автоматически зафиксируется
		if e := tx.Model(&worker).Where("uuid = ?", worker.Uuid).Select("*").UpdateColumns(&worker).Error; e != nil {
			return e // откат транзакции
		}
		return nil
	})
	return e
}

// Метод обновления статуса online worker
func (worker *SlaveWorker) UpdateOnline(db *gorm.DB) error {
	// Создаем транзакцию на добавление новой записи в БД
	// Вызов транзакции
	e := db.Transaction(func(tx *gorm.DB) error {
		// Обновление колонки online не трогая другие
		if e := db.Model(&SlaveWorker{}).
			Where("uuid = ?", worker.Uuid).
			UpdateColumn("online", false).Error; e != nil {
			return e // откат транзакции
		}
		return nil
	})
	return e
}

// Структура описания задачи
type Task struct {
	ID         uint   `gorm:"primaryKey" json:"id"`
	Uuid       string `gorm:"type:uuid;index" json:"Uuid"`                       // Уникальный генерируемый идентификатор связи задачи с воркером
	Status     string `gorm:"type:varchar(100);not null" json:"Status"`          // Статус задачи new|delegated|done|error
	Priority   int32  `gorm:"type:integer;not null;default:5" json:"Priority"`   // Приоритет задачи от 0 до 5 где 0 самый высокий приоритет
	WorkerTask any    `gorm:"serializer:json;not null;default:'{}'" json:"Task"` // Json объект описания задачи
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeletedAt  gorm.DeletedAt
}

// Структура задачи воркера которая передается при запросе
// и хранится в БД задач в виде json объекта
// для данной задачи необходимо передать
// конфигурацию источника миграции
// конфигурацию назначения миграции
// название бакета источника
// кол-во батчей (1 бачт 1000 объектов)
// название бакета назначения
type WorkerMigrateTask struct {
	TaskName    string            `json:"TaskName"`    // Название задачи
	Description string            `json:"Description"` // Описание задачи
	From        global.Connection `json:"From"`        // Конфигурация подключения источника миграции
	FromBucket  string            `json:"FromBucket"`  // Название бакета источника миграции
	To          global.Connection `json:"To"`          // Конфигурация подключения назначения миграции
	ToBucket    string            `json:"ToBucket"`    // Название бакета назначения миграции
	Prefix      string            `json:"Prefix"`      // Префикс по которому выполняется считывание объектов для миграции
}

// Метод получения списка задач для указанного worker
func (task *Task) GetWorkerNewTaskList(db *gorm.DB) ([]Task, error) {
	tasks := []Task{}
	rows, e := db.Model(&task).Where("uuid = ?", task.Uuid).Where("status = ?", "new").Rows()
	if e != nil {
		return tasks, e
	}
	defer rows.Close()

	for rows.Next() {
		var entry Task
		e = db.ScanRows(rows, &entry)
		if e != nil {
			logrus.Error("Ошибка получения данных из БД: ", e.Error())
			continue
		}
		tasks = append(tasks, entry)
	}

	return tasks, nil
}

// Метод получения списка всех задач
func (task *Task) GetTaskList(db *gorm.DB) ([]Task, error) {
	tasks := []Task{}
	rows, e := db.Model(&task).Rows()
	if e != nil {
		return tasks, e
	}
	defer rows.Close()

	for rows.Next() {
		var entry Task
		db.ScanRows(rows, &entry)
		tasks = append(tasks, entry)
	}

	return tasks, nil
}

// Метод получения задач конкретного worker
func (task *Task) GetWorkerTasks(db *gorm.DB, wurker_uuid string) ([]Task, error) {
	tasks := []Task{}
	rows, e := db.Model(&task).Where("uuid = ?", wurker_uuid).Rows()
	if e != nil {
		return tasks, e
	}
	defer rows.Close()
	for rows.Next() {
		var entry Task
		db.ScanRows(rows, &entry)
		tasks = append(tasks, entry)
	}

	return tasks, nil
}

// Метод получения задачи
func (task *Task) GetTask(db *gorm.DB) (Task, error) {
	t := Task{}
	e := db.Where("id = ?", task.ID).First(&t).Error
	if e != nil {
		return t, e
	}
	return t, nil
}

// Метод добавления новой задачи
func (task *Task) AddNewTask(db *gorm.DB) error {
	// Создаем транзакцию на добавление новой записи в БД
	// Вызов транзакции
	e := db.Transaction(func(tx *gorm.DB) error {
		// Создание новой записи в БД
		// Если всё успешно, транзакция автоматически зафиксируется
		if e := tx.Model(&task).Create(&task).Error; e != nil {
			return e // откат транзакции
		}
		return nil
	})
	return e
}

// Метод изменения статуса задачи
func (task *Task) ChangeTaskStatus(db *gorm.DB) error {
	// Создаем транзакцию на добавление новой записи в БД
	// Вызов транзакции
	e := db.Transaction(func(tx *gorm.DB) error {
		// Создание новой записи в БД
		// Если всё успешно, транзакция автоматически зафиксируется
		if e := tx.Model(&task).Update("status", task.Status).Error; e != nil {
			return e // откат транзакции
		}
		return nil
	})
	return e
}

// Метод смены UUID worker задачи
func (task *Task) ChangeTaskWorkerUuid(db *gorm.DB) error {
	// Создаем транзакцию на добавление новой записи в БД
	// Вызов транзакции
	e := db.Transaction(func(tx *gorm.DB) error {
		// Создание новой записи в БД
		// Если всё успешно, транзакция автоматически зафиксируется
		if e := tx.Model(&task).Update("uuid", task.Uuid).Error; e != nil {
			return e // откат транзакции
		}
		return nil
	})
	return e
}

// Метод удаления задачи
func (task *Task) RemoveTask(db *gorm.DB) error {
	// Создаем транзакцию на добавление новой записи в БД
	// Вызов транзакции
	e := db.Transaction(func(tx *gorm.DB) error {
		// Создание новой записи в БД
		// Если всё успешно, транзакция автоматически зафиксируется
		if e := tx.Model(&task).Delete("id", task.ID).Error; e != nil {
			return e // откат транзакции
		}
		return nil
	})
	return e
}

// Структура описания отчета MigrateReport
type MigrateReport struct {
	ID           uint     `gorm:"primaryKey;"`
	Uuid         string   `gorm:"not null;type:uuid;uniqueIndex" json:"Uuid"`                    // UUID воркера
	TaskID       uint     `gorm:"not null;type:integer" json:"TaskID"`                           // ID задачи
	TotalObjects uint     `gorm:"not null;type:integer" json:"TotalObjects"`                     // Обчее количество обработанных объектов
	ErrorObjects uint     `gorm:"not null;type:integer" json:"ErrorObjects"`                     // Кол-во объектов обработанных с ошибкой
	DoneObjects  uint     `gorm:"not null;type:integer" json:"DoneObjects"`                      // Кол-во успешно мигрированных объектов
	Errors       []string `gorm:"serializer:json;not null;type:json;default:'[]'" json:"Errors"` // Массив ключей объектов миграция которых закончилась с ошибкой
}

// Создание новой или обновление существуючей записи репорта выполнения операции миграции объектов
func (report *MigrateReport) CreateOrUpdateMigrateReport(db *gorm.DB) error {
	errorsJSON, e := json.Marshal(report.Errors)
	if e != nil {
		return e
	}
	// Пытаемся обновить существующую запись
	result := db.Model(&MigrateReport{}).
		Where("uuid = ?", report.Uuid).
		Updates(map[string]interface{}{
			"task_id":       report.TaskID,
			"total_objects": report.TotalObjects,
			"error_objects": report.ErrorObjects,
			"done_objects":  report.DoneObjects,
			"errors":        errorsJSON,
		})

	if result.Error != nil {
		return result.Error
	}

	// Если запись не была обновлена (не существует) - создаем новую
	if result.RowsAffected == 0 {
		return db.Create(report).Error
	}

	return nil
}
