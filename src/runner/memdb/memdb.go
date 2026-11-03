package memdb

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/tidwall/buntdb"
)

type MemoryDatabase struct {
	DB *buntdb.DB
}

// Пакет управления inmemory key:value db
func (memdb *MemoryDatabase) InitDB() (*buntdb.DB, error) {
	/*config := buntdb.Config{
		SyncPolicy:           buntdb.Never,      // Отключаем сброс на ФС нам он не нужен
		AutoShrinkDisabled:   false,             // Разрешаем автоматическое сжатие БД
		AutoShrinkPercentage: 100,               // Параметры сжатия - сжатие начнется только при 100% увеличении от предыдучего сжатия
		AutoShrinkMinSize:    100 * 1024 * 1024, // 100 МБ - сжатие начнется только после того как размер БД станет >=100 МБ

	}*/
	// Создание экземпляра подключения к БД в оперативной памяти
	// индекс b-tree стандартный по ключу
	db, e := buntdb.Open(":memory:")
	if e != nil {
		return db, e
	}
	memdb.DB = db
	return db, nil
}

// Метод создания записи/записей в БД
// Выполнение добавление записей в БД выполняется без выхода по ошибке для всех
// переданных записей
func (memdb *MemoryDatabase) CreateOrUpdateRecords(kv map[string]string) {
	for key := range kv {
		e := memdb.DB.Update(func(tx *buntdb.Tx) error {
			_, _, e := tx.Set(key, kv[key], nil)
			return e
		})
		if e != nil {
			logrus.Errorf("Ошибка транзакции добавления записи в memdb: %s", e.Error())
		}
		continue
	}
	logrus.Info("Выполненно добавление записей в memdb")
}

// Метод извлечения значения по ключу
func (memdb *MemoryDatabase) GetValueByKey(key string) (string, error) {
	if key == "" {
		return "", fmt.Errorf("Ошибка key является пустой строкой")
	}

	var value string
	e := memdb.DB.View(func(tx *buntdb.Tx) error {
		if val, e := tx.Get(key); e != nil {
			return e
		} else {
			value = val
			return nil
		}
	})
	return value, e
}

// Метод получения кол-ва ключей в БД
func (memdb *MemoryDatabase) GetDBKeyCount() (int, error) {
	var keycount int
	e := memdb.DB.View(func(tx *buntdb.Tx) error {
		count, e := tx.Len()
		if e != nil {
			return e
		}
		keycount = count
		return nil
	})
	if e != nil {
		return keycount, e
	}
	return keycount, nil
}

// Метод сброса БД (удаления всех ключей из БД)
func (memdb *MemoryDatabase) DropDB() error {
	// Закрываем текучую БД для сброса
	e := memdb.DB.Close()
	if e != nil {
		return e
	}
	// ДАем время на очистку
	time.Sleep(5 * time.Second)
	// Переинициализируем БД
	_, e = memdb.InitDB()
	if e != nil {
		return e
	}
	return nil
}
