package db

import (
	"database/sql"
	"errors"
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Создание экземпляра подключения к БД
// данный экземпляр будет передаваться во все функции
// и выступать основным экземпляром подключения к БД
func MakeDBConnection(t string, sqlite_db_name any, db_host any, db_user any,
	db_password any, db_dbname any, db_port any, db_ssl any) (*gorm.DB, *sql.DB, error) {
	if t == "postgress" {
		db, dbo, e := pgdbConnect(db_host.(string),
			db_user.(string),
			db_password.(string),
			db_dbname.(string),
			db_port.(string), db_ssl.(bool))
		return db, dbo, e
	}

	if t == "sqlite" {
		db, e := sqlitedbConnect(sqlite_db_name.(string))
		return db, nil, e
	}

	return nil, nil, errors.New("тип БД не определен")
}

// Метод подключения к БД postgres
// возвращает ссылку на тип *gorm.DB и состояние
func pgdbConnect(host string, user string, password string, dbname string, port string, ssl bool) (*gorm.DB, *sql.DB, error) {

	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		host, user, password, dbname, port, func() string {
			if ssl {
				return "enable"
			}
			return "disable"
		}(),
	)
	db, e := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if e != nil {
		return nil, nil, e
	}

	dbo, e := db.DB()
	if e != nil {
		return nil, nil, e
	}
	return db, dbo, nil
}

// Метод подключения к БД sqlite
// возвращает ссылку на тип *gorm.DB и состояние
func sqlitedbConnect(dbname string) (db *gorm.DB, err error) {
	// _journal_mode=WAL — включает режим Write-Ahead Log (чтение не блокирует запись)
	// _synchronous=NORMAL — значительно снижает количество жестких fsync на диск без потери надежности в WAL
	// ?_busy_timeout=5000&_synchronous=NORMAL&_journal_mode=WAL&cache=private
	dsn := dbname + "?_busy_timeout=5000&_synchronous=FULL&_journal_mode=DELETE&cache=private"

	db, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	//Ограничиваем пул соединений рантайма Go
	sqlDB, err := db.DB()
	if err == nil {
		// Для SQLite в Go ВСЕГДА должно быть только 1 открытое соединение на запись!
		// Это гарантирует, что Go сам выстроит запросы в очередь горутин,
		// вместо того чтобы плодить потоки ОС в CGO.
		sqlDB.SetMaxOpenConns(10)
		sqlDB.SetMaxIdleConns(10)
	}

	return db, nil
}
