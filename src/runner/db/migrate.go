package db

import "gorm.io/gorm"

// Метод инициализации БД с миграцией структур
func InitDb(db *gorm.DB) error {
	// Создание таблиз из пустых структур/моделей
	e := db.AutoMigrate(&SlaveWorker{}, &Task{}, &MigrateReport{})
	if e != nil {
		return e
	}
	return nil
}
