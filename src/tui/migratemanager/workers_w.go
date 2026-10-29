package migratemanager

import (
	"encoding/json"
	"fmt"
	"minioclient/runner/db"
	"minioclient/utils"
	"os"
	"strconv"
	"time"

	"github.com/epiclabs-io/winman"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"golang.org/x/term"
	"gorm.io/gorm"
)

// Метод создания и обновления таблици workers
func makeWorkersTable(wm *winman.Manager, app *tview.Application,
	table *tview.Table, logger chan string, d *gorm.DB) error {
	refresh = true
	table.Clear()
	table.SetBorders(true)
	table.SetBorder(true)
	table.SetTitle("WORKERS")
	table.SetSelectable(true, true)

	// Заполнение колонок
	cols := []string{"UUID", "HOSTNAME", "MAC", "IP", "OS", "ONLINE", "BUSY", "TASK ID", "TASK STATUS"}
	for i, cell := range cols {
		table.SetCell(0, i, tview.NewTableCell(cell).SetTextColor(tcell.ColorYellow).SetAlign(tview.AlignCenter))
	}

	w := db.SlaveWorker{}
	workers, e := w.ListAllWorkers(d)
	if e != nil {
		logger <- "Ошибка запроса списка workers: " + e.Error()
		return e
	}
	if len(workers) == 0 {
		return nil
	}

	for x, worker := range workers {
		table.SetCell(x+1, 0,
			tview.NewTableCell(worker.Uuid).
				SetClickedFunc(func() bool {
					cell := table.GetCell(x+1, 0)
					showWorkerContext(wm, app, cell, logger, d)
					return true
				}).
				SetTextColor(tcell.ColorYellow).
				SetAlign(tview.AlignCenter))
		table.SetCell(x+1, 1,
			tview.NewTableCell(worker.HostName).
				SetTextColor(tcell.ColorWhite).
				SetAlign(tview.AlignCenter))
		table.SetCell(x+1, 2,
			tview.NewTableCell(worker.MacAddress).
				SetTextColor(tcell.ColorWhite).
				SetAlign(tview.AlignCenter))
		table.SetCell(x+1, 3,
			tview.NewTableCell(worker.IpAddress).
				SetTextColor(tcell.ColorWhite).
				SetAlign(tview.AlignCenter))
		table.SetCell(x+1, 4,
			tview.NewTableCell(worker.OsType).
				SetTextColor(tcell.ColorWhite).
				SetAlign(tview.AlignCenter))
		table.SetCell(x+1, 5,
			tview.NewTableCell(func() string {
				if worker.Online {
					return "🟢"
				}
				return "🔻"
			}()).
				SetTextColor(tcell.ColorWhite).
				SetAlign(tview.AlignCenter))
		table.SetCell(x+1, 6,
			tview.NewTableCell(func() string {
				if worker.Status {
					return "🟦"
				}
				return "🟩"
			}()).
				SetTextColor(tcell.ColorWhite).
				SetAlign(tview.AlignCenter))
		table.SetCell(x+1, 7,
			tview.NewTableCell("🖱️ "+strconv.Itoa(int(worker.CurrentTaskId))).
				SetClickedFunc(func() bool {
					cell := table.GetCell(x+1, 7)
					showWorkersTaskIdContext(wm, app, cell, logger, uint(worker.CurrentTaskId), d)
					return true
				}).
				SetBackgroundColor(tcell.ColorGray).
				SetTextColor(tcell.ColorWhite).
				SetAlign(tview.AlignCenter))
		table.SetCell(x+1, 8,
			tview.NewTableCell(func() string {
				if worker.TaskStatus == "done" {
					return "✅"
				}
				if worker.TaskStatus == "Prepaire task for launche" {
					return "⏳"
				}
				if worker.TaskStatus == "in-progress" {
					return "🚀"
				}
				if worker.TaskStatus == "error" {
					return "❌"
				}
				return "-"
			}()).
				SetTextColor(tcell.ColorWhite).
				SetAlign(tview.AlignCenter))
	}

	// Примитив для вызова контекстного меню по нажатию enter
	table.SetSelectedFunc(func(row, column int) {
		cell := table.GetCell(row, column)
		switch {
		case column == 7:
			taskid := func() uint {
				i, e := strconv.Atoi(cell.Text)
				if e != nil {
					logger <- "Ошибка определения ID задачи"
					return 0
				}
				return uint(i)
			}()
			showWorkersTaskIdContext(wm, app, cell, logger, taskid, d)
		default:
			return
		}
	})

	refresh = false
	return nil
}

// метод создания новой задачи
func showCreateTask(wm *winman.Manager, app *tview.Application, logger chan string, d *gorm.DB) error {
	w, h, _ := term.GetSize(int(os.Stdout.Fd()))
	m := wm.NewWindow()
	m.SetDraggable(true)
	m.SetBorder(true)
	m.SetResizable(true)
	m.SetRect((w*25)/100, h/4, w-(w*60)/100, 21)

	workerTask := db.WorkerMigrateTask{}
	workerTask.TaskName = "Migrate bucket" // Указание имени задачи по которой воркер определяет ее тип
	task := db.Task{}

	// Создание формы
	form := tview.NewForm()
	form.AddDropDown("Воркер", func() []string {
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
	form.AddDropDown("Приоритет задачи", []string{"1", "2", "3", "4", "5"}, 2, func(option string, optionIndex int) {
		i, e := strconv.Atoi(option)
		if e != nil {
			task.Priority = 3
		}
		task.Priority = int32(i)
	})
	form.AddInputField("Описание", "", 0, nil, func(text string) {
		workerTask.Description = text
	})
	form.GetFormItem(2).(*tview.InputField).SetPlaceholder("Описание задачи").SetPlaceholderTextColor(tcell.Color100)
	form.AddDropDown("Источник миграции", func() []string {
		// Формируем список доступных подключений из конфигураионного файла
		var items []string
		cfg, _, e := utils.Readcfg()
		if e != nil {
			logger <- "Ошибка чтения файла конфигурации: " + e.Error()
			return items
		}
		for _, connection := range cfg.Connections {
			items = append(items, connection.Name)
		}
		return items
	}(), 0, func(option string, optionIndex int) {
		workerTask.From.Name = option
	})
	form.AddInputField("Бакет источника", "", 0, nil, func(text string) {
		workerTask.FromBucket = text
	})
	form.GetFormItem(4).(*tview.InputField).SetPlaceholder("Название бакета источника миграции").SetPlaceholderTextColor(tcell.Color100)
	form.AddInputField("Префикс источника", "", 0, nil, func(text string) {
		workerTask.Prefix = text
	})
	form.GetFormItem(5).(*tview.InputField).SetPlaceholder("Префикс или оставить пустым").SetPlaceholderTextColor(tcell.Color100)
	form.AddDropDown("Назначение миграции", func() []string {
		// Формируем список доступных подключений из конфигураионного файла
		var items []string
		cfg, _, e := utils.Readcfg()
		if e != nil {
			logger <- "Ошибка чтения файла конфигурации: " + e.Error()
			return items
		}
		for _, connection := range cfg.Connections {
			items = append(items, connection.Name)
		}
		return items
	}(), 0, func(option string, optionIndex int) {
		workerTask.To.Name = option
	})
	form.AddInputField("Бакет назначения", "", 0, nil, func(text string) {
		workerTask.ToBucket = text
	})
	form.GetFormItem(7).(*tview.InputField).SetPlaceholder("Название бакета назначения или оставить пустым").SetPlaceholderTextColor(tcell.Color100)
	form.AddButton("Создать задачу", func() {
		task.WorkerTask = &workerTask // Сохроняем ссылку на объект для возможности последуючего извлечения и редактирвоания совйств объекта
		logger <- "Создание новой задачи для worker: " + task.Uuid
		e := addNewTask(d, logger, &task)
		if e != nil {
			m.SetBorderColor(tcell.ColorRed)
		} else {
			m.SetBorderColor(tcell.ColorGreen)
		}
	})
	form.AddButton("Отмена", func() {
		wm.RemoveWindow(m)
	})

	m.SetRoot(form)
	m.SetTitle(" Создание задачи ")
	m.AddButton(&winman.Button{
		Symbol:  '❌',
		OnClick: func() { wm.RemoveWindow(m) },
	})
	m.Show()
	app.SetFocus(m)
	return nil
}

// Метод отображения подробной информации о задаче
func showTaskInfo(wm *winman.Manager, app *tview.Application, taskid uint,
	logger chan string, d *gorm.DB) error {
	if taskid == 0 {
		return nil
	}
	w, h, _ := term.GetSize(int(os.Stdout.Fd()))
	m := wm.NewWindow()
	m.SetDraggable(true)
	m.SetBorder(true)
	m.SetResizable(true)
	m.SetRect((w*25)/100, h/4, 60, 25)

	t := db.Task{ID: taskid}

	task, e := t.GetTask(d)
	if e != nil {
		logger <- "Ошибка получения списка задач: " + e.Error()
		return e
	}

	// Формирование окна формы свойств задачи
	form := tview.NewForm()

	form.AddTextView("UUID", task.Uuid, 0, 1, true, true)
	form.AddTextView("STATUS", func() string {
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
	}(), 0, 1, true, true)
	form.AddTextView("PRIORITY", strconv.Itoa(int(task.Priority)), 0, 1, true, true)
	form.AddTextView("CREATE AT", task.CreatedAt.String(), 0, 1, true, true)
	form.AddTextView("UPODATE AT", task.UpdatedAt.String(), 0, 1, true, true)
	form.AddTextView("TASK", func() string {
		b, e := json.MarshalIndent(task.WorkerTask, " ", " ")
		if e != nil {
			logger <- "Ошибка сериализации данных: " + e.Error()
			return ""
		}
		return string(b)
	}(), 0, 12, true, true)

	m.SetRoot(form)
	m.SetTitle(fmt.Sprintf(" Свойства задачи %d ", taskid))
	m.AddButton(&winman.Button{
		Symbol:  '❌',
		OnClick: func() { wm.RemoveWindow(m) },
	})
	m.Show()
	app.SetFocus(m)
	return nil
}

// Метод отображения контекстного меню задачи при нажатии на ID задачи
func showWorkersTaskIdContext(wm *winman.Manager, app *tview.Application, cell *tview.TableCell,
	logger chan string, taskid uint, d *gorm.DB) {
	x, y, _ := cell.GetLastPosition()
	//w, _, _ := term.GetSize(int(os.Stdout.Fd()))
	m := wm.NewWindow()
	m.SetDraggable(false)
	m.SetBorder(true)
	m.SetResizable(false)
	m.SetRect(x, y, 25, 10)

	list := tview.NewList()
	list.AddItem("Свойства задачи", "", ' ', func() {
		e := showTaskInfo(wm, app, taskid, logger, d)
		if e != nil {
			logger <- "Ошибка получения свойств задачи: " + e.Error()
		}
		wm.RemoveWindow(m)
	})
	list.AddItem("Пауза выполнения", "", ' ', func() {})
	list.AddItem("[red]Прерывание задачи[white]", "", ' ', func() {})
	list.AddItem("Закрыть меню", "", ' ', func() { wm.RemoveWindow(m) })
	list.SetCurrentItem(-1)

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

// Метод отображения меню workera
func showWorkerContext(wm *winman.Manager, app *tview.Application, cell *tview.TableCell,
	logger chan string, d *gorm.DB) {
	x, y, _ := cell.GetLastPosition()
	//w, _, _ := term.GetSize(int(os.Stdout.Fd()))
	m := wm.NewWindow()
	m.SetDraggable(false)
	m.SetBorder(true)
	m.SetResizable(false)
	m.SetRect(x, y, 25, 10)

	list := tview.NewList()
	list.AddItem("Отчет выполнения", "", ' ', func() { wm.RemoveWindow(m) })
	list.AddItem("[red]Удалить[white]", "", ' ', func() { removeWorker(cell.Text, logger, d); wm.RemoveWindow(m) })
	list.AddItem("Закрыть меню", "", ' ', func() { wm.RemoveWindow(m) })
	list.SetCurrentItem(-1)

	m.SetRoot(list)
	m.SetTitle(" меню worker ")
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

// Метод удаления worker
func removeWorker(uuid string, logger chan string, d *gorm.DB) error {

	logger <- "Удаление worker: " + uuid
	task := db.Task{
		Uuid: uuid,
	}

	// Сначала проверяем что у удаляемого worker нет активных задач
	workertasks, e := task.GetWorkerTasks(d, uuid)
	for _, t := range workertasks {
		if t.Status == "in-progress" || t.Status == "delegated" || t.Status == "Prepaire task for launche" {
			logger <- fmt.Sprintf("Ошибка удаления worker: %s - у воркера есть активные задачи", uuid)
			return fmt.Errorf("Ошибка удаления worker: %s - у воркера есть активные задачи", uuid)
		}
	}

	worker := db.SlaveWorker{
		Uuid: uuid,
	}

	e = worker.DeleteWorker(d)
	if e != nil {
		logger <- "Ошибка удаления worker: " + e.Error()
		return e
	}

	return nil
}
