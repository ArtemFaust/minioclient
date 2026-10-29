package migratemanager

import (
	"context"
	"database/sql"
	"minioclient/runner/db"
	"os"
	"time"

	"github.com/epiclabs-io/winman"
	"github.com/rivo/tview"
	"golang.org/x/term"
	"gorm.io/gorm"
)

var refresh bool // Зачита от тыканья
type dbconnection struct {
	Db    *gorm.DB
	RawDb *sql.DB
	Error error
}

var gormdb dbconnection // Глобальное подключение к БД

// Ссылки на текучие активные окна что бы не порождать новые окна
type currentwindows struct {
	Manager *winman.WindowBase // Управление воркерами
	Tasks   *winman.WindowBase // Управление задачами
}

var windows currentwindows

// Метод создания окна управления slave воркерами миграции
func MakeWindow(wm *winman.Manager, app *tview.Application, logger chan string, ctx context.Context) {
	if windows.Manager != nil {
		app.SetFocus(windows.Manager)
		return
	}

	// Создаем глобальное подключение к БД
	if gormdb.Db == nil {
		dbcfg, _, e := ReadDbCfg()
		if e != nil {
			logger <- "Ошибка чтения конфигурации подключения к БД: " + e.Error()
			return
		}

		// Проверяем существование файла БД для sqlite
		if dbcfg.Db.Dbtype == "sqlite" {
			if _, e := os.Stat(dbcfg.Db.Sqlitedbname); e != nil {
				logger <- "Ошибка файил базы данных не найден по указанному пути"
				return
			}
		}

		// Создаем подключение к БД
		gormdb.Db, gormdb.RawDb, gormdb.Error = db.MakeDBConnection(
			dbcfg.Db.Dbtype, dbcfg.Db.Sqlitedbname, dbcfg.Db.Pg_db_host, dbcfg.Db.Pg_db_user,
			dbcfg.Db.Pg_db_password, dbcfg.Db.Pg_db_dbname, dbcfg.Db.Pg_db_port, dbcfg.Db.Pg_db_ssl)
		if gormdb.Error != nil {
			logger <- "Ошибка подключенияк БД: " + gormdb.Error.Error()
			return
		}

	}

	localctx, cancel := context.WithCancel(context.Background())

	w, h, _ := term.GetSize(int(os.Stdout.Fd()))
	m := wm.NewWindow()
	m.SetDraggable(true)
	m.SetBorder(true)
	m.SetResizable(true)
	m.SetRect((w*5)/100, h/4, w-(w*10)/100, h-(h*50)/100)
	table := tview.NewTable()
	table.SetFixed(1, 0)
	e := makeWorkersTable(wm, app, table, logger, gormdb.Db)
	if e != nil {
		logger <- "Ошибка обновления данных workers: " + e.Error()
	}
	m.SetRoot(table)

	m.SetTitle(" Менеджер миграций ")
	m.AddButton(&winman.Button{
		Symbol: '❌',
		OnClick: func() {
			cancel()
			windows.Manager = nil
			wm.RemoveWindow(m)
		},
	})
	m.AddButton(&winman.Button{
		Symbol: '🔄',
		OnClick: func() {
			if !refresh {
				makeWorkersTable(wm, app, table, logger, gormdb.Db)
			}
		},
	})
	m.AddButton(&winman.Button{
		Symbol: '➕',
		OnClick: func() {
			showCreateTask(wm, app, logger, gormdb.Db)
		},
	})
	m.AddButton(&winman.Button{
		Symbol: '🟰',
		OnClick: func() {
			if windows.Tasks != nil {
				app.SetFocus(windows.Tasks)
				return
			}
			getAllTaskList(wm, app, logger, gormdb.Db)
		},
	})

	// Запуск фонового обновления таблици workers
	go updateWorkersTable(wm, table, logger, ctx, app, localctx)

	m.Show()
	app.SetFocus(m)
	windows.Manager = m
}

// Метод фонового обновления таблици workers
func updateWorkersTable(wm *winman.Manager, table *tview.Table, logger chan string,
	globalctx context.Context, app *tview.Application, localctx context.Context) {
	for {
		time.Sleep(3 * time.Second)
		select {
		case <-globalctx.Done():
			return
		case <-localctx.Done():
			return
		default:
			e := makeWorkersTable(wm, app, table, logger, gormdb.Db)
			if e != nil {
				logger <- "Ошибка обновления таблици workers: " + e.Error()
				continue
			}
			app.Draw()
		}
	}
}
