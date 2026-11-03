package migratemanager

import (
	"errors"
	"fmt"
	"minioclient/runner/db"
	"minioclient/utils"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/epiclabs-io/winman"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/sirupsen/logrus"
	"golang.org/x/term"
	"gorm.io/gorm"
)

// метод отображения списка всех задач конкретного воркера
func getAllTaskList(wm *winman.Manager, app *tview.Application, logger chan string, d *gorm.DB) error {

	w, h, _ := term.GetSize(int(os.Stdout.Fd()))
	m := wm.NewWindow()
	m.SetDraggable(true)
	m.SetBorder(true)
	m.SetResizable(true)
	m.SetRect((w*10)/100, h/4, w-(w*20)/100, h-(h*50)/100)

	table := tview.NewTable()
	table.SetFixed(1, 0)

	e := makeTaskListTable(wm, app, table, logger, d)
	if e != nil {
		logger <- "Ошибка обновления данных задач: " + e.Error()
	}

	m.SetRoot(table)
	m.SetTitle(" Список задач ")
	m.AddButton(&winman.Button{
		Symbol:  '❌',
		OnClick: func() { windows.Tasks = nil; wm.RemoveWindow(m) },
	})
	m.AddButton(&winman.Button{
		Symbol:  '🔄',
		OnClick: func() { table.Clear(); makeTaskListTable(wm, app, table, logger, d) },
	})

	// Примитив для вызова контекстного меню по нажатию enter
	table.SetSelectedFunc(func(row, column int) {
		cell := table.GetCell(row, column)
		uuid := table.GetCell(row, 1).Text

		switch {
		case column == 0:
			taskid := func() uint {
				i, e := strconv.Atoi(cell.Text)
				if e != nil {
					logger <- "Ошибка определения ID задачи"
					return 0
				}
				return uint(i)
			}()
			showListTaskStatusContext(wm, app, cell, logger, taskid, uuid, table, d)
		default:
			return
		}
	})

	m.Show()
	app.SetFocus(m)
	windows.Tasks = m
	return nil
}

// Метод создания таблици списка задач
func makeTaskListTable(wm *winman.Manager, app *tview.Application, table *tview.Table,
	logger chan string, d *gorm.DB) error {
	t := db.Task{}
	tasks, e := t.GetTaskList(d)
	if e != nil {
		logger <- "Ошибка получения списка задач: " + e.Error()
		return e
	}

	table.SetBorders(true)
	table.SetBorder(true)
	table.SetTitle("")
	table.SetSelectable(true, true)

	// Заполнение колонок
	cols := []string{"ID", "WORKER", "STATUS", "PRIORITY", "CREATE AT", "UPDATE AT"}
	for i, cell := range cols {
		table.SetCell(0, i, tview.NewTableCell(cell).SetTextColor(tcell.ColorYellow).SetAlign(tview.AlignCenter))
	}

	// сортируем список объектов (что бы не пригал)
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].ID < tasks[j].ID
	})

	for x, task := range tasks {
		table.SetCell(x+1, 0,
			tview.NewTableCell("🖱️ "+strconv.Itoa(int(task.ID))).
				SetClickedFunc(func() bool {
					cell := table.GetCell(x+1, 0)
					uuid := table.GetCell(x+1, 1).Text
					if cell.Text == "" || len(strings.Split(cell.Text, " ")) == 0 {
						logger <- "Ошибка определения ID задачи: " + e.Error()
						return true
					}
					id := strings.Split(cell.Text, " ")[1]
					i, e := strconv.Atoi(id)
					if e != nil {
						logger <- "Ошибка определения ID задачи: " + e.Error()
						return true
					}
					showListTaskStatusContext(wm, app, cell, logger, uint(i), uuid, table, d)
					return true
				}).
				SetTextColor(tcell.ColorYellow).
				SetAlign(tview.AlignCenter))
		table.SetCell(x+1, 1,
			tview.NewTableCell(task.Uuid).
				SetTextColor(tcell.ColorWhite).
				SetAlign(tview.AlignCenter))
		table.SetCell(x+1, 2,
			tview.NewTableCell(func() string {
				if task.Status == "done" {
					return "✅"
				}
				if task.Status == "Prepaire task for launche" {
					return "⏳"
				}
				if task.Status == "in-progress" {
					return "🚀"
				}
				if task.Status == "error" {
					return "❌"
				}
				if task.Status == "delegated" {
					return "➡️"
				}
				if task.Status == "new" {
					return "🕒"
				}
				return "-"
			}()).
				SetTextColor(tcell.ColorWhite).
				SetAlign(tview.AlignCenter))
		table.SetCell(x+1, 3,
			tview.NewTableCell(strconv.Itoa(int(task.Priority))).
				SetTextColor(tcell.ColorWhite).
				SetAlign(tview.AlignCenter))
		table.SetCell(x+1, 4,
			tview.NewTableCell(task.CreatedAt.String()).
				SetTextColor(tcell.ColorWhite).
				SetAlign(tview.AlignCenter))
		table.SetCell(x+1, 5,
			tview.NewTableCell(task.UpdatedAt.String()).
				SetTextColor(tcell.ColorWhite).
				SetAlign(tview.AlignCenter))
	}

	return nil
}

// Исполнительные методы - непосредственное применение/созранение изменений
// Метод занесения задачи в БД - допилить
func addNewTask(d *gorm.DB, logger chan string, task *db.Task) error {
	task.Status = "new"

	// Считываем конфиг
	config, _, e := utils.Readcfg()
	if e != nil {
		logger <- "Ошибка чтения конфигурационного файла: " + e.Error()
		return e
	}

	// Приводим тип один раз в самом начале, чтобы избежать паники и дублирования
	migrateTask, ok := task.WorkerTask.(*db.WorkerMigrateTask)
	if !ok {
		logger <- "Ошибка: задача не является типом *db.WorkerMigrateTask"
		return errors.New("неверный тип задачи миграции")
	}

	// Флаги для проверки, нашли ли мы нужные подключения
	var fromFound, toFound bool

	// Ищем оба подключения за ОДИН проход по циклу (повышает производительность)
	for _, c := range config.Connections {
		if !fromFound && c.Name == migrateTask.From.Name {
			migrateTask.From = c
			logger <- "Данные подключения источника миграции извлечены"
			fromFound = true
		}
		if !toFound && c.Name == migrateTask.To.Name {
			migrateTask.To = c
			logger <- "Данные подключения назначения миграции извлечены"
			toFound = true
		}
		// Если всё нашли, можно выходить из цикла раньше времени
		if fromFound && toFound {
			break
		}
	}

	// Проверяем результаты поиска после завершения цикла
	if !fromFound {
		logger <- "Ошибка извлечения данных подключения источника"
		return errors.New("Ошибка извлечения данных подключения источника")
	}

	if !toFound {
		logger <- "Ошибка извлечения данных подключения назначения"
		return errors.New("Ошибка извлечения данных подключения назначения")
	}

	e = task.AddNewTask(d)
	if e != nil {
		logrus.Error("Ошибка создания новой задачи: ", e.Error())
		return e
	}

	logger <- fmt.Sprintf("Созданна новая задача для worker: %s Приоритет задачи: %d", task.Uuid, task.Priority)
	return nil
}

// Метод удаления задания из очереди
func removeTaskFromQueue(wm *winman.Manager, app *tview.Application, taskid uint, logger chan string, d *gorm.DB,
	table *tview.Table) error {
	t := db.Task{
		ID: taskid,
	}
	// Сначала проверяем статус задачи - удалить можно только задачу в состоянии new
	if task, e := t.GetTask(d); e != nil {
		logger <- "Ошибка удаления задачи из списка: " + e.Error()
		return e
	} else {
		if task.Status == "new" {
			if e := t.RemoveTask(d); e != nil {
				logger <- "Ошибка удаления задачи из списка: " + e.Error()
				return e
			}
			logger <- fmt.Sprintf("Удаление задачи %d выполненно", taskid)
			table.Clear()
			makeTaskListTable(wm, app, table, logger, d)
			return nil
		} else {
			logger <- "Ошибка удалять можно задачи только в статусе новая"
			return errors.New("Ошибка удалять можно задачи только в статусе новая")
		}
	}
}

// Метод отображения контекстного меню задачи при нажатии на статус задачи
func showListTaskStatusContext(wm *winman.Manager, app *tview.Application, cell *tview.TableCell,
	logger chan string, taskid uint, uuid string, table *tview.Table, d *gorm.DB) {
	x, y, _ := cell.GetLastPosition()
	//w, _, _ := term.GetSize(int(os.Stdout.Fd()))
	m := wm.NewWindow()
	m.SetDraggable(false)
	m.SetBorder(true)
	m.SetResizable(false)
	m.SetRect(x, y, 25, 14)

	list := tview.NewList()
	list.AddItem("Свойства задачи", "", ' ', func() {
		e := showTaskInfo(wm, app, taskid, logger, d)
		if e != nil {
			logger <- "Ошибка получения свойств задачи: " + e.Error()
		}
		wm.RemoveWindow(m)
	})
	list.AddItem("Отчет выполнения", "", ' ', func() {
		showTaskMigrateReport(wm, app, uuid, taskid, logger, d)
		wm.RemoveWindow(m)
	})
	list.AddItem("Переназначить", "", ' ', func() {
		e := changeTaskRunnerWorker(wm, app, taskid, logger, d, table)
		if e != nil {
			makeErrorModal(wm, app, e.Error())
		}
		wm.RemoveWindow(m)
	})
	list.AddItem("[yellow]Отмена задачи[white]", "", ' ', func() {
		e := removeTaskFromQueue(wm, app, taskid, logger, d, table)
		if e != nil {
			makeErrorModal(wm, app, e.Error())
		}
		wm.RemoveWindow(m)
	})
	list.AddItem("[red]Принудительно fail[white]", "", ' ', func() {
		e := setTaskErrorState(wm, app, taskid, logger, d, table)
		if e != nil {
			makeErrorModal(wm, app, e.Error())
		}
		wm.RemoveWindow(m)
	})
	list.AddItem("Закрыть меню", "", ' ', func() { wm.RemoveWindow(m) })
	list.SetCurrentItem(0)

	m.SetRoot(list)
	m.SetTitle(" меню задачи ")
	m.Show()
	app.SetFocus(m)
	m.SetBlurFunc(func() {
		go func() {
			// Даем winman и tview несколько миллисекунд,
			// чтобы завершить внутренние процессы смены фокуса
			time.Sleep(10 * time.Millisecond)
			if app.GetFocus() != m {
				// Безопасно удаляем окно и обновляем интерфейс
				app.QueueUpdateDraw(func() {
					wm.RemoveWindow(m)
				})
			}
		}()
	})
}

// Метод принудительной смены состояния задачи
// Данный метод необходим что бы позволить удалить воркер с зависшими задачами
func setTaskErrorState(wm *winman.Manager, app *tview.Application, taskid uint, logger chan string, d *gorm.DB,
	table *tview.Table) error {
	t := db.Task{
		ID: taskid,
	}
	// Сначала проверяем статус задачи - удалить можно только задачу в состоянии new
	if task, e := t.GetTask(d); e != nil {
		logger <- "Ошибка удаления задачи из списка: " + e.Error()
		return e
	} else {
		if task.Status == "done" {
			logger <- "Ошибка нельзя выставить fail выполненной задаче"
			return errors.New("Ошибка нельзя выставить fail выполненной задаче")
		}
		task.Status = "error"
		e = task.ChangeTaskStatus(d)
		if e != nil {
			logger <- "Ошибка смены статуса задачи: " + e.Error()
			return e
		}
		logger <- "Смена статуса задачи выполненна"
		return nil
	}
}

// Метод переназначения задачи другому воркеру
// если задача в состоянии - ожидание (new) ее можно переназначить лдругому worker
func changeTaskRunnerWorker(wm *winman.Manager, app *tview.Application, taskid uint, logger chan string, d *gorm.DB,
	table *tview.Table) error {
	w, h, _ := term.GetSize(int(os.Stdout.Fd()))
	m := wm.NewWindow()
	m.SetDraggable(true)
	m.SetBorder(true)
	m.SetResizable(false)
	m.SetRect((w*30)/100, h/3, 60, 8)

	t := db.Task{ID: taskid}
	task, e := t.GetTask(d)
	if e != nil {
		logger <- "Ошибка получения задачи: " + e.Error()
		return e
	}

	if task.Status != "new" {
		logger <- fmt.Sprintf("Ошибка смены worker для задачи: %d - смену воркера можно проводить только для новой задачи", task.ID)
		return fmt.Errorf("Ошибка смены worker для задачи: %d - смену воркера можно проводить только для новой задачи", task.ID)
	}

	form := tview.NewForm()
	form.AddDropDown("Переназначить на: ", func() []string {
		workerslist := []string{}
		w := db.SlaveWorker{}
		workers, e := w.ListAllWorkers(d)
		if e != nil {
			logger <- "Ошибка формирования списка доступных worker"
			return workerslist
		}
		for _, worker := range workers {
			workerslist = append(workerslist, worker.Uuid)
		}
		return workerslist
	}(), 0, func(option string, optionIndex int) {
		task.Uuid = option
	})
	form.AddButton("Применить", func() {
		e := task.ChangeTaskWorkerUuid(d)
		if e != nil {
			logger <- "Ошибка смены worker для задачи: " + e.Error()
			makeErrorModal(wm, app, "Ошибка смены worker для задачи: "+e.Error())
			return
		}
		logger <- fmt.Sprintf("Выполненна смена worker для задачи ID: %d", task.ID)
	})
	form.AddButton("Отмена", func() { wm.RemoveWindow(m) })

	m.SetRoot(form)
	m.SetTitle(fmt.Sprintf(" Смена worker для задачи %d ", taskid))
	m.AddButton(&winman.Button{
		Symbol:  '❌',
		OnClick: func() { wm.RemoveWindow(m) },
	})
	m.Show()
	return nil
}
