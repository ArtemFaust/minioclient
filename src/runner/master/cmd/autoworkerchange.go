package cmd

import (
	"context"
	"fmt"
	"minioclient/runner/db"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// Метод просмотра задач и инициализации смены worker для задач
// если назначенный worker offline
func AutoWorkerChanger(ctx context.Context, d *gorm.DB) error {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		logrus.Info("Запуск метода переназначения задач workers")
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			// Получаем список задач
			t := db.Task{}
			tasks, e := t.GetTaskList(d)
			if e != nil {
				logrus.Errorf("Ошибка получения списка задач: %s", e.Error())
				continue
			}
			if len(tasks) == 0 {
				continue
			}

			// Получаем список workers
			w := db.SlaveWorker{}
			workers, e := w.ListAllWorkers(d)
			if e != nil {
				logrus.Error("Ошибка получения списка workers: ", e.Error())
				continue
			}
			if len(workers) == 0 {
				continue
			}

			for _, task := range tasks {
			workerselect:
				for _, worker := range workers {
					// Если UUID задачи совпадает с UUID назначенного воркера и воркер не онлайн
					// выполняем переназначение задачи на любой воркер который онлайн и ни чем не занят
					if worker.Uuid == task.Uuid && task.Status == "new" && !worker.Online {
						logrus.Infof("Обноруженна задача назначенная недоступному worker. ID задачи: %d worker: %s", task.ID, worker.Uuid)
						// Определяем доступный свободный worker
					changeworkerloop:
						for _, w := range workers {
							if w.Online && !w.Status {
								e := changeTaskWorker(d, task.ID, w.Uuid)
								if e != nil {
									logrus.Errorf("Ошибка смены worker для задачи %d. Ошибка: %s", task.ID, e.Error())
									continue changeworkerloop
								}
								logrus.Infof("Выполненна смена worker для задачи %d - новый worker: %s", task.ID, w.Uuid)
								break workerselect
							}
						}
						logrus.Warnf("Не удалось выполнить смену worker для задачи: %d - нет свободных workers", task.ID)
					}
				}
			}
		}
	}
}

// Метод смены воркера для задачи если исполнитель находится offline а задача в очереди
func changeTaskWorker(d *gorm.DB, taskid uint, newworkeruuid string) error {
	logrus.Infof("Смена worker для задачи: %d", taskid)
	t := db.Task{
		ID: taskid,
	}

	task, e := t.GetTask(d)
	if e != nil {
		return e
	}

	if task.Status != "new" {
		return fmt.Errorf(`Ошибка смены worker для задачи: %d
		смену воркера можно проводить только для новой задачи - статус задачи: %s`, task.ID, task.Status)
	}

	task.Uuid = newworkeruuid

	e = task.ChangeTaskWorkerUuid(d)
	if e != nil {
		return fmt.Errorf("Ошибка смены worker для задачи: %s", e.Error())
	}

	return nil

}
