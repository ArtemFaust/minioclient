package cmd

import (
	"context"
	"minioclient/runner/db"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// Фоновый процесс проверки online сщстояния workers
func HealthCheckWorkers(ctx context.Context, d *gorm.DB) error {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		logrus.Info("Запуск метода проверки доступности workers")
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			healthCheck(d)
		}
	}
}

// Метод проверки доступности workers
// Проверяет время последнего обновления данных о состоянии воркера
// если воркер не обновлял свои данные более чем 2 минут то считаем его недоступным
func healthCheck(d *gorm.DB) error {
	w := db.SlaveWorker{}
	workers, e := w.ListAllWorkers(d)
	if e != nil {
		logrus.Error("Ошибка получения списка workers: ", e.Error())
		return e
	}

	if len(workers) == 0 {
		return nil
	}

	// Вычисляем порог времени один раз перед циклом
	threshold := time.Now().Add(-4 * time.Minute)

	for i := range workers {
		// Если последнее обновление было раньше, чем 2 минуты назад
		if workers[i].UpdatedAt.Before(threshold) {
			logrus.Infof("Worker %s - недоступен по времени последнего обновления", workers[i].Uuid)

			workers[i].Online = false
			if e := workers[i].UpdateOnline(d); e != nil {
				logrus.Errorf("Ошибка обновления статуса доступности worker %s: %v", workers[i].Uuid, e)
				continue
			}
		}
	}
	return nil
}
