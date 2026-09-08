package tui

import (
	"encoding/json"
	"fmt"
	bucketoperations "minioclient/bucket_operations"
	filedialog "minioclient/tui/customwidgets/filedialog"
	"os"
	"strconv"
	"time"

	"github.com/epiclabs-io/winman"
	"github.com/gdamore/tcell/v2"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/lifecycle"
	"github.com/minio/minio-go/v7/pkg/notification"
	"github.com/rivo/tview"
	"golang.org/x/term"
)

// Глобальные статусы

// Блокиратор нужен для того что бы в процессе получения бакетов не вызвали операцию refresh
// так как мы теряем управление над выполняющейся горутиной
// так называемая защита от дребезка (когда пользователь ебашит по кнопке не думая)
var GET_BUCKETS_WORK_STATUS bool

// Метод создания окна BUCKET LIST
func makeBucketListWindow(wm *winman.Manager, minioClient *minio.Client, app *tview.Application,
	footer *winman.WindowBase, logger chan string, interactive bool) {
	bucketlist := wm.NewWindow()  // Создание нового окна
	bucketlist.SetDraggable(true) // Делаем окно перемечаемым
	bucketlist.SetResizable(true) // Делаем окно маштабируемым
	bucketlist.SetTitleColor(tcell.Color101)
	bucketlist.SetBorderColor(tcell.Color101) // Цвет рамки
	bucketlist.SetTitle("BUCKETS LIST")       // Устанавливаем заголовок

	// Добавляем кнопку закрытия окна
	bucketlist.AddButton(&winman.Button{
		Symbol:  '❌',
		OnClick: func() { app.Stop() },
	})
	// Добавляем кнопку максимализации окна
	bucketlist.AddButton(&winman.Button{
		Symbol: '🔼',
		OnClick: func() {
			func() {
				if bucketlist.IsMaximized() {
					bucketlist.GetButton(1).Symbol = '🔼'
					bucketlist.Restore()
				} else {
					bucketlist.GetButton(1).Symbol = '🔽'
					bucketlist.Maximize()
				}
			}()
		},
	})

	// Добавляем кнопу refresh
	bucketlist.AddButton(&winman.Button{
		Symbol: '🔄',
		OnClick: func() {
			// Если уже не выполняется операция получения бакетов
			// можно выполнить операцию refresh
			if !GET_BUCKETS_WORK_STATUS {
				bucketlist.GetRoot().(*tview.List).Clear()
				bucketlist.SetRoot(getBucketList(minioClient, wm, app, footer, logger, bucketlist, interactive))
			}
		},
	})

	// Добавляем кнопку операций
	bucketlist.AddButton(&winman.Button{
		Symbol: '🟰',
		OnClick: func() {
			list := bucketlist.GetRoot()
			var bucket_name string
			if list.(*tview.List).GetItemCount() == 0 {
				bucket_name = ""
			} else {
				bucket_name, _ = list.(*tview.List).GetItemText(list.(*tview.List).GetCurrentItem())
			}

			x, y, _, _ := bucketlist.GetRect()
			modal := wm.NewWindow()
			modallist := tview.NewList().
				AddItem("Создать", "", '➕', func() {
					createNewBucket(minioClient, wm, app, bucketlist, modal, footer, logger, bucketlist, interactive)
					modal.Hide()
				}).SetShortcutColor(tcell.NewRGBColor(0, 0, 0)).
				AddItem("Удалить", "", '➖', func() {
					if bucket_name != "" {
						deleteBucket(minioClient, wm, app, bucketlist, modal, bucket_name, footer, logger, bucketlist, interactive)
					}
					modal.Hide()
				}).SetShortcutColor(tcell.NewRGBColor(0, 0, 0)).
				AddItem("Свойства", "", '❔', func() {
					if bucket_name != "" {
						getBucketInfo(minioClient, wm, app, bucket_name, logger)
					}
					modal.Hide()
				}).SetShortcutColor(tcell.NewRGBColor(0, 0, 0)).
				AddItem("Закрыть меню", "", '❌', func() { modal.Hide() }).SetShortcutColor(tcell.NewRGBColor(0, 0, 0))
			modal.SetRect(x+10, y, 20, 10)
			modal.SetModal(true)
			modal.SetRoot(modallist)
			app.SetFocus(modal)
			modal.Show()
		},
	})

	bucketlist.SetRoot(getBucketList(minioClient, wm, app, footer, logger, bucketlist, interactive)) // Получаем список бакетов

	_, h, _ := term.GetSize(int(os.Stdout.Fd())) // Высота терминала
	_, _, _, fh := footer.GetRect()              // Высота footer
	bucketlist.SetRect(0, 0, 40, h-fh-2)         // Свойства окна
	bucketlist.Show()                            // Показываем окно
	app.SetFocus(bucketlist)
}

// Метод создания списка бакетов
func getBucketList(minioClient *minio.Client, wm *winman.Manager, app *tview.Application,
	footer *winman.WindowBase, logger chan string, bucketlist *winman.WindowBase, interactive bool) *tview.List {
	// Прогресс выполнения операции
	pm, ch := progress(wm, app, "Запрос списка бакетов")

	list := tview.NewList()
	list.SetSecondaryTextColor(tcell.ColorSeaGreen)
	logger <- "Запрос списков бакетов"

	// Обновление списка выполняем в фоне
	go func(pm *winman.WindowBase, list *tview.List, logger chan string) {
		GET_BUCKETS_WORK_STATUS = true // Глобально указываем что операция получения бакетов выполняется
		defer func() {
			pm.SetBorderColor(tcell.ColorGreen)
			close(ch)
			time.Sleep(1 * time.Second)
			wm.RemoveWindow(pm)
			GET_BUCKETS_WORK_STATUS = false // Указываем что операция завершилась
		}()

		buckets, e := bucketoperations.ListBuckets(minioClient, nil)
		if e != nil {
			makeErrorModal(wm, app, e.Error())
			logger <- e.Error()
			return
		}

		count := 0
		bucketlist.SetBorderColor(tcell.ColorYellow)
		for _, b := range buckets {
			count++
			ch <- fmt.Sprintf("Полученно объектов: %v", count)
			list.AddItem(b.Name, b.CreationDate.Format("2006-01-02"), '📁', func() {
				makeObjectsListWindow(wm, minioClient, b.Name, "", "", app, footer, logger, interactive)
			}).SetShortcutColor(tcell.NewRGBColor(0, 0, 0))
		}
		bucketlist.SetBorderColor(tcell.Color101)
		logger <- "Запрос бакетов выполнен успешно"
	}(pm, list, logger)

	return list
}

// Метод создания нового бакета вызываемый из контекстного меню
func createNewBucket(minioClient *minio.Client, wm *winman.Manager, app *tview.Application, window *winman.WindowBase,
	modal *winman.WindowBase, footer *winman.WindowBase,
	logger chan string, bucketlist *winman.WindowBase, interactive bool) {
	region := "us-east-1"
	new_bucket_name := ""
	ol := false

	w, h, _ := term.GetSize(int(os.Stdout.Fd()))
	m := wm.NewWindow()
	m.SetDraggable(true)
	m.SetBorder(true)
	m.SetRect(w/3, h/4, 50, 11)

	form := tview.NewForm().
		AddInputField("Название бакета:", "", 20, nil, func(text string) {
			new_bucket_name = text
		}).
		AddInputField("Регион:", "us-east-1", 20, nil, func(text string) {
			region = text
		}).
		AddCheckbox("Включить блокировку объектов:", false, func(checked bool) {
			ol = checked
		}).
		AddButton("Создать", func() {
			logger <- "Создание нового бакета"
			e := bucketoperations.MakeBucket(minioClient, &new_bucket_name, &region, &ol)
			if e != nil {
				// Показываем модальное окно с ошибкой
				makeErrorModal(wm, app, e.Error())
				logger <- fmt.Sprintf("Ошибка создания бакета %s: %s"+new_bucket_name, e.Error())
			} else {
				window.SetRoot(getBucketList(minioClient, wm, app, footer, logger, bucketlist, interactive))
				logger <- fmt.Sprintf("Создание бакета %s выполненно успешно", new_bucket_name)
				wm.RemoveWindow(m)
			}
		}).
		AddButton("Отмена", func() {
			wm.RemoveWindow(m)
		})

	m.SetRoot(form)
	m.SetTitle("Создание бакета")
	m.Show()
	app.SetFocus(m)

	modal.Hide()
}

// Метод удаления бакета вызываемый из контекстного меню
func deleteBucket(minioClient *minio.Client, wm *winman.Manager, app *tview.Application, window *winman.WindowBase,
	modal *winman.WindowBase, bucket_name string, footer *winman.WindowBase,
	logger chan string, bucketlist *winman.WindowBase, interactive bool) {
	w, h, _ := term.GetSize(int(os.Stdout.Fd()))
	m := wm.NewWindow()
	m.SetDraggable(true)
	m.SetBorder(true)
	m.SetRect(w/3, h/4, 26, 5)
	form := tview.NewForm().
		AddButton("Удалить", func() {
			logger <- "Удаление бакета " + bucket_name
			e := bucketoperations.DeleteBucket(minioClient, &bucket_name)
			if e != nil {
				// Показываем модальное окно с ошибкой
				makeErrorModal(wm, app, e.Error())
				logger <- fmt.Sprintf("Ошибка удаления бакета %s: %s", bucket_name, e.Error())
			} else {
				window.SetRoot(getBucketList(minioClient, wm, app, footer, logger, bucketlist, interactive))
				logger <- fmt.Sprintf("Удаления бакета %s выполненно успешно", bucket_name)
			}
			wm.RemoveWindow(m)
		}).
		AddButton("Отмена", func() {
			wm.RemoveWindow(m)
		})

	m.SetRoot(form)
	m.SetTitle("Удаление бакета " + bucket_name)
	m.Show()
	app.SetFocus(m)

	modal.Hide()
}

// Метод получения информации о бакете вызываемый из контекстного меню
func getBucketInfo(minioClient *minio.Client, wm *winman.Manager, app *tview.Application, bucket_name string,
	logger chan string) {
	logger <- "Запрос свойств бакета"
	bi := bucketoperations.BucketInfo{
		Name: &bucket_name,
	}

	bi.Make(minioClient)
	makeBucketInfoModal(minioClient, wm, app, logger, bi)
}

// Создание модального окна свойст бакета
func makeBucketInfoModal(minioClient *minio.Client, wm *winman.Manager, app *tview.Application,
	logger chan string, bi bucketoperations.BucketInfo) {

	w, h, _ := term.GetSize(int(os.Stdout.Fd()))
	m := wm.NewWindow()
	m.SetDraggable(true)
	m.SetResizable(true)
	m.SetBorder(true)
	m.SetRect(w/3, h/4, 70, 40)
	m.AddButton(&winman.Button{
		Symbol:  '❌',
		OnClick: func() { wm.RemoveWindow(m) },
	})

	// Параметры бакета которые применяются к нему по нажатию кнопки применить
	versioning := bi.Versioning.Enabled() // Реактивная переменная хранения статуса версионирования
	var policy string                     // Реактивная переменная хранения политики бакета
	var lc *lifecycle.Configuration       // Параметры LC
	var notif notification.Configuration  // Параметры нотификации

	tpol := tview.NewTextArea()   // Политики бакета
	ttlc := tview.NewTextArea()   // LC бакета
	tnotif := tview.NewTextArea() // Параметры нотификации

	form := tview.NewForm().
		AddInputField("Название бакета:", *bi.Name, 20, nil, nil).
		AddCheckbox("Статус версионирования:", bi.Versioning.Enabled(), func(checked bool) {
			versioning = checked
		}).
		AddCheckbox("Статус LC", !bi.LifeCycle.Empty(), nil).
		AddInputField("Размещение:", bi.Location, 20, nil, nil).
		AddButton("Применить", func() {
			status := true
			// Смена параметров версионирования
			e := bi.ChangeVersioningSettings(minioClient, versioning)
			if e != nil {
				status = false
				logger <- fmt.Sprintf("Ошибка смены статуса версионирования для бакета %s : %s", *bi.Name, e.Error())
			} else {
				logger <- fmt.Sprintf("Переключение статуса версионирования для бакета %s выполнено", *bi.Name)
			}

			// Установка политики бакета
			e = bi.ChangeBucketPolicy(minioClient, policy)
			if e != nil {
				tpol.SetBorderColor(tcell.ColorRed)
				status = false
				logger <- fmt.Sprintf("Ошибка смены политики для бакета %s : %s", *bi.Name, e.Error())
			} else {
				tpol.SetBorderColor(tcell.ColorGreen)
				logger <- fmt.Sprintf("Переключение политики для бакета %s выполнено", *bi.Name)
			}

			// Установка LC
			e = bi.ChangeBucketLc(minioClient, lc)
			if e != nil {
				ttlc.SetBorderColor(tcell.ColorRed)
				status = false
				logger <- fmt.Sprintf("Ошибка смены lc для бакета %s : %s", *bi.Name, e.Error())
			} else {
				ttlc.SetBorderColor(tcell.ColorGreen)
				logger <- fmt.Sprintf("Переключение lc для бакета %s выполнено", *bi.Name)
			}

			// Установка NOTOFICATION
			e = bi.ChangeNotificationConfig(minioClient, notif)
			if e != nil {
				tnotif.SetBorderColor(tcell.ColorRed)
				status = false
				logger <- fmt.Sprintf("Ошибка смены notification для бакета %s : %s", *bi.Name, e.Error())
			} else {
				tnotif.SetBorderColor(tcell.ColorGreen)
				logger <- fmt.Sprintf("Переключение notification для бакета %s выполнено", *bi.Name)
			}
			if !status {
				m.SetBorderColor(tcell.ColorRed)
			} else {
				m.SetBorderColor(tcell.ColorGreen)
			}

		}).
		AddButton("🔼 policy", func() {
			fileselectB(wm, app, logger, tpol, "Выбор политики бакета "+*bi.Name)
		}).
		AddButton("🔼 lc", func() {
			fileselectB(wm, app, logger, ttlc, "Выбор LC бакета "+*bi.Name)
		}).
		AddButton("🔼 notify", func() {
			fileselectB(wm, app, logger, tnotif, "ВЫбор нотификации бакета "+*bi.Name)
		})
	// Политики
	tpol.SetText(func() string {
		policy = bi.Policy // Реактивная переменная сохраняет исходное значение
		return policy      // Возвращаем форматированный объект
	}(), false)
	//tpol.SetTextColor(tcell.ColorWhite)
	//tpol.SetDynamicColors(true)
	tpol.SetBorder(true)
	tpol.SetTitle("Policy")
	//tpol.SetScrollable(true)
	tpol.SetChangedFunc(func() {
		// Валидация что текст является json
		b := json.Valid([]byte(tpol.GetText()))
		if !b {
			tpol.SetBorderColor(tcell.ColorRed)
		} else {
			tpol.SetBorderColor(tcell.ColorGreen)
			policy = tpol.GetText() // Меняем реактивную переменную
		}
	})

	// LC
	ttlc.SetText(func() string {
		if bi.LifeCycle == nil {
			return ""
		}
		// Выполняем десериализацию lc
		b, e := json.MarshalIndent(bi.LifeCycle, " ", " ")
		lc = bi.LifeCycle // Реативная переменная = текучему lc
		// Если возникает ошибка десериализацию
		if e != nil {
			logger <- fmt.Sprintf("Ошибка десериализацию LC бакета: %s", e.Error())
			return "" // Возвращаем пустую строку
		}
		// Выполняем форматирование строки
		s, e := strconv.Unquote(string(b))
		// Если возникает ошибка
		if e != nil {
			//logger <- fmt.Sprintf("Ошибка форматирования LC бакета: %s", e.Error())
			return string(b) // Возвращаем строку без изменения
		}
		return s // Возвращаем форматированную строку
	}(), false)
	//ttlc.SetTextColor(tcell.ColorWhite)
	//ttlc.SetDynamicColors(true)
	ttlc.SetBorder(true)
	ttlc.SetTitle("LC")
	//ttlc.SetScrollable(true)
	ttlc.SetChangedFunc(func() {
		// Валидация что текст является json
		if !json.Valid([]byte(ttlc.GetText())) {
			ttlc.SetBorderColor(tcell.ColorRed)
		} else {
			e := json.Unmarshal([]byte(ttlc.GetText()), &lc)
			if e != nil {
				ttlc.SetBorderColor(tcell.ColorRed)
			}
			ttlc.SetBorderColor(tcell.ColorGreen)
		}
	})

	// Параметры notification
	tnotif.SetText(func() string {
		//Выполняем десериализацию
		b, e := json.MarshalIndent(bi.NotificationConfiguration, " ", " ")
		notif = bi.NotificationConfiguration // Реативной переменной присваиваем текучее значение
		// Если возникает ошибка
		if e != nil {
			logger <- fmt.Sprintf("Ошибка обработки notification бакета: %s", e.Error())
			return "" // Возвращаем пустую строку
		}

		return string(b)
	}(), false)
	//tnotif.SetTextColor(tcell.ColorWhite)
	//tnotif.SetDynamicColors(true)
	tnotif.SetBorder(true)
	tnotif.SetTitle("NOTIFICATION")
	//tnotif.SetScrollable(true)
	tnotif.SetChangedFunc(func() {
		// Валидация что текст является json
		if !json.Valid([]byte(tnotif.GetText())) {
			tnotif.SetBorderColor(tcell.ColorRed)
		} else {
			e := json.Unmarshal([]byte(tnotif.GetText()), &notif)
			if e != nil {
				tnotif.SetBorderColor(tcell.ColorRed)
			}
			tnotif.SetBorderColor(tcell.ColorGreen)
		}
	})

	grid := tview.NewGrid().
		SetRows(-2, -1, -1, -1).
		SetColumns(0).
		SetBorders(false).
		AddItem(form, 0, 0, 1, 1, 0, 0, false).
		AddItem(tpol, 1, 0, 1, 1, 0, 0, false).
		AddItem(ttlc, 2, 0, 1, 1, 0, 0, false).
		AddItem(tnotif, 3, 0, 1, 1, 0, 0, false)

	m.SetRoot(grid)
	m.SetTitle("Свойства бакета " + *bi.Name)
	m.Show()
	app.SetFocus(m)
}

// Метод выбора файла конфигурации
func fileselectB(wm *winman.Manager, app *tview.Application, logger chan string, t *tview.TextArea, title string) {
	m := wm.NewWindow()
	m.SetDraggable(true)
	m.SetResizable(true)
	m.SetBorder(true)
	m.SetRect(10, 10, 40, 20)
	m.SetTitle(title)
	m.AddButton(&winman.Button{
		Symbol:  '❌',
		OnClick: func() { wm.RemoveWindow(m) },
	})

	homedir, e := os.UserHomeDir()
	if e != nil {
		homedir = "."
	}

	fd := filedialog.NewFileDialog(homedir, func(filePath string) {
		b, e := os.ReadFile(filePath)
		if e != nil {
			logger <- fmt.Sprintf("Ошибка чтения файла: %s", e.Error())
		} else {
			t.SetText(string(b), false)
		}
		wm.RemoveWindow(m)
	})

	m.SetRoot(fd)
	m.Show()
	app.SetFocus(m)
}
