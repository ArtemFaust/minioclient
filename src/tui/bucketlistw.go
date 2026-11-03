package tui

import (
	"context"
	"encoding/json"
	"fmt"
	bucketoperations "minioclient/bucket_operations"
	"minioclient/global"
	objectoperations "minioclient/object_operations"
	topicoperations "minioclient/topicoperations"
	filedialog "minioclient/tui/customwidgets/filedialog"
	"minioclient/utils"
	"os"
	"strconv"
	"time"

	"github.com/epiclabs-io/winman"
	"github.com/gdamore/tcell/v2"
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
// Возвращает указатель на созданный окно
func makeBucketListWindow(wm *winman.Manager, client *global.GlobalClient, app *tview.Application,
	footer *winman.WindowBase, logger chan string, interactive bool, usessl *bool, region *string) *winman.WindowBase {
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
			if !GET_BUCKETS_WORK_STATUS && client.MinioClient != nil && client.ClientCfg != nil {
				bucketlist.GetRoot().(*tview.List).Clear()
				bucketlist.SetRoot(getBucketList(client, wm, app, footer, logger, bucketlist, interactive))
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
					if client.MinioClient != nil && client.ClientCfg != nil {
						createNewBucket(client, wm, app, bucketlist, modal, footer, logger, bucketlist, interactive)
					}
					modal.Hide()
				}).SetShortcutColor(tcell.NewRGBColor(0, 0, 0)).
				AddItem("Удалить", "", '➖', func() {
					if bucket_name != "" && client.MinioClient != nil && client.ClientCfg != nil {
						deleteBucket(client, wm, app, bucketlist, modal, bucket_name, footer, logger, bucketlist, interactive)
					}
					modal.Hide()
				}).SetShortcutColor(tcell.NewRGBColor(0, 0, 0)).
				AddItem("Свойства", "", '❔', func() {
					if bucket_name != "" && client.MinioClient != nil && client.ClientCfg != nil {
						getBucketInfo(client, wm, app, bucket_name, logger)
					}
					modal.Hide()
				}).SetShortcutColor(tcell.NewRGBColor(0, 0, 0)).
				AddItem("Мигрировать", "", '🔄', func() {
					if client.MinioClient != nil && client.ClientCfg != nil {
						initMigrateBucketOperation(wm, app, bucket_name, logger, client, usessl, region)
					}
					modal.Hide()
				}).
				AddItem("Топики", "", '📝', func() {
					showTopicList(wm, app, logger, client)
					modal.Hide()
				}).
				AddItem("Нотификации", "", '🔔', func() {
					if bucket_name != "" && client.MinioClient != nil && client.ClientCfg != nil {
						showNotification(client, wm, app, bucket_name, logger)
						modal.Hide()
					}
				}).
				AddItem("Life Cycle", "", '🔁', func() {
					if bucket_name != "" && client.MinioClient != nil && client.ClientCfg != nil {
						showLifeCycle(client, wm, app, bucket_name, logger)
						modal.Hide()
					}
				}).
				AddItem("Политика", "", '🚨', func() {
					if bucket_name != "" && client.MinioClient != nil && client.ClientCfg != nil {
						showPolicy(client, wm, app, bucket_name, logger)
						modal.Hide()
					}
				}).
				AddItem("Закрыть меню", "", '❌', func() { modal.Hide() }).SetShortcutColor(tcell.NewRGBColor(0, 0, 0))
			modal.SetRect(x+10, y, 22, 15)
			modal.SetModal(true)
			modal.SetRoot(modallist)
			modal.Show()
			app.SetFocus(modal)
		},
	})

	bucketlist.SetRoot(getBucketList(client, wm, app, footer, logger, bucketlist, interactive)) // Получаем список бакетов

	_, h, _ := term.GetSize(int(os.Stdout.Fd())) // Высота терминала
	_, _, _, fh := footer.GetRect()              // Высота footer
	bucketlist.SetRect(0, 0, 40, h-fh-2)         // Свойства окна
	bucketlist.Show()                            // Показываем окно
	app.SetFocus(bucketlist)
	return bucketlist
}

// Метод создания списка бакетов
func getBucketList(client *global.GlobalClient, wm *winman.Manager, app *tview.Application,
	footer *winman.WindowBase, logger chan string, bucketlist *winman.WindowBase, interactive bool) *tview.List {
	list := tview.NewList()
	if client.ClientCfg != nil && client.MinioClient != nil {
		// Прогресс выполнения операции
		pm, ch := progress(wm, app, "Запрос списка бакетов")

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

			buckets, e := bucketoperations.ListBuckets(client, nil)
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
					makeObjectsListWindow(wm, client, b.Name, "", "", app, footer, logger, interactive)
				}).SetShortcutColor(tcell.NewRGBColor(0, 0, 0))
			}
			bucketlist.SetBorderColor(tcell.Color101)
			logger <- "Запрос бакетов выполнен успешно"
		}(pm, list, logger)
	}
	return list
}

// Метод создания нового бакета вызываемый из контекстного меню
func createNewBucket(client *global.GlobalClient, wm *winman.Manager, app *tview.Application, window *winman.WindowBase,
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
			go func() {
				logger <- "Создание нового бакета"
				e := bucketoperations.MakeBucket(client, &new_bucket_name, &region, &ol)
				if e != nil {
					// Показываем модальное окно с ошибкой
					makeErrorModal(wm, app, e.Error())
					logger <- fmt.Sprintf("Ошибка создания бакета %s: %s"+new_bucket_name, e.Error())
				} else {
					window.SetRoot(getBucketList(client, wm, app, footer, logger, bucketlist, interactive))
					logger <- fmt.Sprintf("Создание бакета %s выполненно успешно", new_bucket_name)
					wm.RemoveWindow(m)
				}
			}()
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
func deleteBucket(client *global.GlobalClient, wm *winman.Manager, app *tview.Application, window *winman.WindowBase,
	modal *winman.WindowBase, bucket_name string, footer *winman.WindowBase,
	logger chan string, bucketlist *winman.WindowBase, interactive bool) {
	w, h, _ := term.GetSize(int(os.Stdout.Fd()))
	m := wm.NewWindow()
	m.SetDraggable(true)
	m.SetBorder(true)
	m.SetRect(w/3, h/4, 26, 5)
	form := tview.NewForm()
	form.AddButton("Удалить", func() {
		logger <- "Удаление бакета " + bucket_name
		go func(wm *winman.Manager) {
			e := bucketoperations.DeleteBucket(client, &bucket_name)
			if e != nil {
				// Показываем модальное окно с ошибкой
				makeErrorModal(wm, app, e.Error())
				logger <- fmt.Sprintf("Ошибка удаления бакета %s: %s", bucket_name, e.Error())
			} else {
				window.SetRoot(getBucketList(client, wm, app, footer, logger, bucketlist, interactive))
				logger <- fmt.Sprintf("Удаления бакета %s выполненно успешно", bucket_name)
			}
			wm.RemoveWindow(m)
		}(wm)
	}).AddButton("Отмена", func() {
		wm.RemoveWindow(m)
	})

	m.SetRoot(form)
	m.SetTitle("Удаление бакета " + bucket_name)
	m.Show()
	app.SetFocus(m)

	modal.Hide()
}

// Метод получения информации о бакете вызываемый из контекстного меню
func getBucketInfo(client *global.GlobalClient, wm *winman.Manager, app *tview.Application, bucket_name string,
	logger chan string) {
	logger <- "Запрос свойств бакета"
	bi := bucketoperations.BucketInfo{
		Name: &bucket_name,
	}

	e := bi.Make(client)
	if e != nil {
		logger <- fmt.Sprintf("Ошибка запроса свойств бакета: %s", e.Error())
		return
	}
	makeBucketInfoModal(client, wm, app, logger, bi)
}

// Создание модального окна свойст бакета
func makeBucketInfoModal(client *global.GlobalClient, wm *winman.Manager, app *tview.Application,
	logger chan string, bi bucketoperations.BucketInfo) {

	w, h, _ := term.GetSize(int(os.Stdout.Fd()))
	m := wm.NewWindow()
	m.SetDraggable(true)
	m.SetResizable(true)
	m.SetBorder(true)
	m.SetRect(w/3, h/4, 47, 19)
	m.AddButton(&winman.Button{
		Symbol:  '❌',
		OnClick: func() { wm.RemoveWindow(m) },
	})

	// Параметры бакета которые применяются к нему по нажатию кнопки применить
	versioning := bi.Versioning.Enabled() // Реактивная переменная хранения статуса версионирования

	form := tview.NewForm().
		AddInputField("Название бакета:", *bi.Name, 20, nil, nil).
		AddCheckbox("Статус версионирования:", bi.Versioning.Enabled(), func(checked bool) {
			versioning = checked
		}).
		AddCheckbox("Статус LC", !bi.LifeCycle.Empty(), nil).
		AddInputField("Размещение:", bi.Location, 20, nil, nil).
		AddInputField("Объектов:", bi.ObjectCount, 20, nil, nil).
		AddInputField("Байт:", bi.BytesUsed, 20, nil, nil).
		AddInputField("Max buckets:", bi.MaxBuckets, 20, nil, nil).
		AddButton("Применить", func() {
			// Смена параметров версионирования
			e := bi.ChangeVersioningSettings(client, versioning)
			if e != nil {
				logger <- fmt.Sprintf("Ошибка смены статуса версионирования для бакета %s : %s", *bi.Name, e.Error())
			} else {
				logger <- fmt.Sprintf("Переключение статуса версионирования для бакета %s выполнено", *bi.Name)
			}
		})

	m.SetRoot(form)
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

// Метод инициализации миграции бакета
func initMigrateBucketOperation(wm *winman.Manager, app *tview.Application,
	sourcebucketname string, logger chan string, fomclient *global.GlobalClient, usessl *bool, region *string) {

	var toendpoint string
	dstbucketname := sourcebucketname
	maxentry := 10

	w, h, _ := term.GetSize(int(os.Stdout.Fd()))
	m := wm.NewWindow()
	m.SetDraggable(true)
	m.SetBorder(true)
	m.SetRect(w/3, h/4, 55, 11)

	form := tview.NewForm().
		AddInputField("Название бакета:", sourcebucketname, 20, nil, func(text string) {
			dstbucketname = text
		}).
		AddDropDown("Кластер назначения:", func() []string {
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
			toendpoint = option
		}).
		AddInputField("Кол-во батчей (1 батч 1000 объектов):", strconv.Itoa(maxentry), 20, nil, func(text string) {
			if th, e := strconv.Atoi(text); e == nil {
				maxentry = th
			}
		}).
		AddButton("Мигрировать", func() {
			// Выполняем миграцию объектов в отдельном потоке
			go func() {
				if dstbucketname == "" {
					logger <- "Ошибка миграции бакета: destination bucket name is empty"
					return
				}
				// Прогресс выполнения операции
				pm, ch := progress(wm, app, "Миграция бакета: "+sourcebucketname)

				// Контекст выполнения операции миграции данных
				ctx, cancel := context.WithCancel(context.Background())
				defer func() {
					close(ch) // закрываем канал
					cancel()  // отменяем контекст выполнения
				}()

				pm.AddButton(&winman.Button{
					Symbol: '❌',
					OnClick: func() {
						cancel() // отменяем контекст выполнения
						wm.RemoveWindow(pm)
					},
				})

				logger <- "Начало миграции бакета: " + sourcebucketname
				e := objectoperations.MigrateObjects(fomclient, sourcebucketname, "",
					toendpoint, usessl, maxentry, dstbucketname, false, true, ctx, cancel, ch, region, false, nil)
				if e != nil {
					logger <- fmt.Sprintf("Ошибка выполнения миграции бакета bucket %s objects: %s", sourcebucketname, e.Error())
					pm.SetBorderColor(tcell.ColorRed) // По окончанию выполнения поля делаем red
					return
				}
				logger <- fmt.Sprintf("Миграция бакета %s выполненна успешна.", sourcebucketname)
				pm.SetBorderColor(tcell.ColorGreen) // По окончанию выполнения поля делаем зелеными
			}()
			wm.RemoveWindow(m)
		}).
		AddButton("Отмена", func() {
			wm.RemoveWindow(m)
		})
	m.SetRoot(form)
	m.SetTitle("Миграция бакета")
	m.AddButton(&winman.Button{
		Symbol:  '❌',
		OnClick: func() { wm.RemoveWindow(m) },
	})
	m.Show()
	app.SetFocus(m)
}

// Метод управления топиками нотификации
func showTopicList(wm *winman.Manager, app *tview.Application, logger chan string, client *global.GlobalClient) {
	w, h, _ := term.GetSize(int(os.Stdout.Fd()))
	m := wm.NewWindow()
	m.SetDraggable(true)
	m.SetBorder(true)
	m.SetResizable(true)
	m.SetRect((w*25)/100, h/4, 55, 10)

	list, e := makeTopicList(wm, app, logger, client)
	if e != nil {
		logger <- "Ошибка формирования списка топиков: " + e.Error()
		return
	}

	m.SetRoot(list)
	m.SetTitle(" Управление топиками нотификации ")
	m.AddButton(&winman.Button{
		Symbol:  '❌',
		OnClick: func() { wm.RemoveWindow(m) },
	})
	m.AddButton(&winman.Button{
		Symbol: '🔄',
		OnClick: func() {
			func() {
				list, e := makeTopicList(wm, app, logger, client)
				if e != nil {
					logger <- "Ошибка формирования списка топиков: " + e.Error()
					return
				}
				m.SetRoot(list)
			}()
		},
	})
	m.AddButton(&winman.Button{
		Symbol:  '➕',
		OnClick: func() { showAddTopic(wm, app, logger, client) },
	})
	m.Show()
	app.SetFocus(m)

}

// Метод создания списка топиков
func makeTopicList(wm *winman.Manager, app *tview.Application, logger chan string, client *global.GlobalClient) (*tview.List, error) {
	list := tview.NewList()
	// Получаем списко топиков
	topiclist, e := topicoperations.GetTopicsList(context.Background(), client.MinioClient)
	if e != nil {
		return list, e
	}

	// Формируем список топиков принадлежачих пользователю
	for _, topik := range topiclist {
		list.AddItem(topik, "", '📝', func() {
			arn, _ := list.GetItemText(list.GetCurrentItem())
			showTopicInfo(wm, app, logger, client, arn)
		})
	}
	return list, nil
}

// Окно создания нового топика
func showAddTopic(wm *winman.Manager, app *tview.Application, logger chan string, client *global.GlobalClient) {
	w, h, _ := term.GetSize(int(os.Stdout.Fd()))
	m := wm.NewWindow()
	m.SetDraggable(true)
	m.SetBorder(true)
	m.SetResizable(true)
	m.SetRect((w*25)/100, h/4, 40, 20)

	topicCfg := topicoperations.TopicConfig{}

	form := tview.NewForm()
	form.AddInputField("Имя топика", "", 0, nil, func(text string) { topicCfg.TopicName = text })
	form.AddInputField("URL подключения", "", 0, nil, func(text string) { topicCfg.PushEndpoint = text })
	form.AddCheckbox("Use ssl", false, func(checked bool) { topicCfg.UseSsl = checked })
	form.AddCheckbox("Verify ssl", false, func(checked bool) { topicCfg.VerifySsl = checked })
	form.AddInputField("HTTP auth user", "", 0, nil, func(text string) { topicCfg.HttpAuthUser = text })
	form.AddInputField("HTTP user password", "", 0, nil, func(text string) { topicCfg.HttpAuthPassword = text })
	form.AddCheckbox("Cloudevents format", false, func(checked bool) { topicCfg.CloudeventsFormat = checked })
	form.AddButton("Создать", func() {
		// Создание топика
		topikcArn, e := topicoperations.CreateOrUpdateTopic(context.Background(), client.MinioClient, topicCfg)
		if e != nil {
			m.SetBorderColor(tcell.ColorRed)
			logger <- fmt.Sprintf("Ошибка создания топика: %s Ошибка: %s", topicCfg.TopicName, e.Error())
		} else {
			m.SetBorderColor(tcell.ColorGreen)
			logger <- fmt.Sprintf("Топик %s успешно создан", topikcArn)
		}
	})
	form.AddButton("Отмена", func() { wm.RemoveWindow(m) })

	m.SetRoot(form)
	m.SetTitle(" Создание нового топика ")
	m.AddButton(&winman.Button{
		Symbol:  '❌',
		OnClick: func() { wm.RemoveWindow(m) },
	})
	m.Show()
	app.SetFocus(m)
}

// Овно информации о топике
func showTopicInfo(wm *winman.Manager, app *tview.Application, logger chan string, client *global.GlobalClient, arn string) {
	w, h, _ := term.GetSize(int(os.Stdout.Fd()))
	m := wm.NewWindow()
	m.SetDraggable(true)
	m.SetBorder(true)
	m.SetResizable(true)
	m.SetRect((w*25)/100, h/4, 40, 20)

	attr, e := topicoperations.GetTopiCInfo(context.Background(), client.MinioClient, arn)
	if e != nil {
		logger <- "Ошибка получения свойств топика: " + arn
		return
	}

	b, e := json.MarshalIndent(attr.Attributes, " ", " ")
	if e != nil {
		logger <- "Ошибка десериализации свойств топика: " + e.Error()
		return
	}

	t := tview.NewTextArea().SetText(string(b), false)
	m.SetRoot(t)

	m.SetTitle(" Свойства топика ")
	m.AddButton(&winman.Button{
		Symbol:  '❌',
		OnClick: func() { wm.RemoveWindow(m) },
	})
	m.AddButton(&winman.Button{
		Symbol: '🔽',
		OnClick: func() {
			currentWorkDir, e := os.Getwd() // Текучая рабочая директория в которую нужно вернуться после окончания операции
			if e != nil {
				logger <- fmt.Sprintf("Ошибка получения текущей рабочей директории: %s", e.Error())
				return
			}

			// Определяем папку загрузки в домашней диреткории пользователя
			homedir, e := os.UserHomeDir()
			if e != nil {
				logger <- fmt.Sprintf("Ошибка получения домашней директории пользователя: %s", e.Error())
				homedir = currentWorkDir
			}

			// Ищем директорию загрузки
			var downloadsDir string
			_, e = os.Stat(homedir + "/Downloads")
			if e != nil {
				_, e = os.Stat(homedir + "/Загрузки")
				if e != nil {
					e = os.Mkdir(homedir+"/Downloads", 0777)
					if e != nil {
						logger <- fmt.Sprintf("Ошибка создания директории Downloads: %s", e.Error())
						return
					}
					downloadsDir = homedir + "/Downloads/"
				}
				downloadsDir = homedir + "/Загрузки/"
			} else {
				downloadsDir = homedir + "/Downloads/"
			}

			// Смена рабочей директории на директорию в которую загружаем объекты
			e = os.Chdir(downloadsDir + "/")
			if e != nil {
				logger <- fmt.Sprintf("Ошибка смены рабочей директории: %s", e.Error())
				return
			}

			e = os.WriteFile(arn+".json", []byte(t.GetText()), 0777)
			if e != nil {
				logger <- fmt.Sprintf("Ошибка созранения файла конфигурации топика %s Ошибка: %s", arn, e.Error())
			}

			// Обратная смена рабочей директории
			e = os.Chdir(currentWorkDir)
			if e != nil {
				logger <- fmt.Sprintf("Ошибка смены рабочей директории: %s", e.Error())
				return
			}
		},
	})
	m.AddButton(&winman.Button{
		Symbol: '➖',
		OnClick: func() {
			defer wm.RemoveWindow(m)
			_, e := topicoperations.RemoveTopic(context.Background(), client.MinioClient, arn)
			if e != nil {
				logger <- fmt.Sprintf("Ошибка удаления топика %s Ошибка: %s", arn, e.Error())
				return
			}
			logger <- fmt.Sprintf("Выполнено удалени топика %s ", arn)
		},
	})
	m.Show()
	app.SetFocus(m)
}

// Метод отображения окна конфигурации нотификации
func showNotification(client *global.GlobalClient, wm *winman.Manager, app *tview.Application, bucket_name string,
	logger chan string) {
	w, h, _ := term.GetSize(int(os.Stdout.Fd()))
	m := wm.NewWindow()
	m.SetDraggable(true)
	m.SetResizable(true)
	m.SetBorder(true)
	m.SetRect(w/3, h/4, 70, 30)

	logger <- "Запрос свойств бакета"
	bi := bucketoperations.BucketInfo{
		Name: &bucket_name,
	}

	e := bi.Make(client)
	if e != nil {
		logger <- fmt.Sprintf("Ошибка запроса свойств бакета: %s", e.Error())
		return
	}

	var notif notification.Configuration // Параметры нотификации

	// Параметры notification
	tnotif := tview.NewTextArea() // Параметры нотификации
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
	tnotif.SetBorder(false)
	//tnotif.SetScrollable(true)
	tnotif.SetChangedFunc(func() {
		// Валидация что текст является json
		if !json.Valid([]byte(tnotif.GetText())) {
			m.SetBorderColor(tcell.ColorRed)
		} else {
			e := json.Unmarshal([]byte(tnotif.GetText()), &notif)
			if e != nil {
				m.SetBorderColor(tcell.ColorRed)
			}
			m.SetBorderColor(tcell.ColorGreen)
		}
	})

	m.SetRoot(tnotif)
	m.SetTitle("Параметры нотификации бакета " + *bi.Name)
	m.AddButton(&winman.Button{
		Symbol:  '❌',
		OnClick: func() { wm.RemoveWindow(m) },
	})
	m.AddButton(&winman.Button{
		Symbol: '💾',
		OnClick: func() {
			e = bi.ChangeNotificationConfig(client, notif)
			if e != nil {
				m.SetBorderColor(tcell.ColorRed)
				logger <- fmt.Sprintf("Ошибка смены notification для бакета %s : %s", *bi.Name, e.Error())
				return
			}
			m.SetBorderColor(tcell.ColorGreen)
			logger <- fmt.Sprintf("Смена notification для бакета %s выполнено", *bi.Name)
		},
	})
	m.Show()
	app.SetFocus(m)
}

// Метод отображения окна LC
func showLifeCycle(client *global.GlobalClient, wm *winman.Manager, app *tview.Application, bucket_name string,
	logger chan string) {
	w, h, _ := term.GetSize(int(os.Stdout.Fd()))
	m := wm.NewWindow()
	m.SetDraggable(true)
	m.SetResizable(true)
	m.SetBorder(true)
	m.SetRect(w/3, h/4, 70, 30)

	logger <- "Запрос свойств бакета"
	bi := bucketoperations.BucketInfo{
		Name: &bucket_name,
	}

	e := bi.Make(client)
	if e != nil {
		logger <- fmt.Sprintf("Ошибка запроса свойств бакета: %s", e.Error())
		return
	}

	var lc *lifecycle.Configuration // Параметры LC
	ttlc := tview.NewTextArea()     // LC бакета
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
	ttlc.SetBorder(false)
	//ttlc.SetScrollable(true)
	ttlc.SetChangedFunc(func() {
		// Валидация что текст является json
		if !json.Valid([]byte(ttlc.GetText())) {
			m.SetBorderColor(tcell.ColorRed)
		} else {
			e := json.Unmarshal([]byte(ttlc.GetText()), &lc)
			if e != nil {
				m.SetBorderColor(tcell.ColorRed)
			}
			m.SetBorderColor(tcell.ColorGreen)
		}
	})

	m.SetRoot(ttlc)
	m.SetTitle("Параметры LC бакета " + *bi.Name)
	m.AddButton(&winman.Button{
		Symbol:  '❌',
		OnClick: func() { wm.RemoveWindow(m) },
	})
	m.AddButton(&winman.Button{
		Symbol: '💾',
		OnClick: func() {
			e = bi.ChangeBucketLc(client, lc)
			if e != nil {
				m.SetBorderColor(tcell.ColorRed)
				logger <- fmt.Sprintf("Ошибка смены lc для бакета %s : %s", *bi.Name, e.Error())
			} else {
				m.SetBorderColor(tcell.ColorGreen)
				logger <- fmt.Sprintf("Смена lc для бакета %s выполнено", *bi.Name)
			}
		},
	})
	m.Show()
	app.SetFocus(m)

}

// Метод отображения окна политик
func showPolicy(client *global.GlobalClient, wm *winman.Manager, app *tview.Application, bucket_name string,
	logger chan string) {
	w, h, _ := term.GetSize(int(os.Stdout.Fd()))
	m := wm.NewWindow()
	m.SetDraggable(true)
	m.SetResizable(true)
	m.SetBorder(true)
	m.SetRect(w/3, h/4, 70, 30)

	logger <- "Запрос свойств бакета"
	bi := bucketoperations.BucketInfo{
		Name: &bucket_name,
	}

	e := bi.Make(client)
	if e != nil {
		logger <- fmt.Sprintf("Ошибка запроса свойств бакета: %s", e.Error())
		return
	}

	var policy string           // Реактивная переменная хранения политики бакета
	tpol := tview.NewTextArea() // Политики бакета

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

	m.SetTitle("Политика бакета " + *bi.Name)
	m.AddButton(&winman.Button{
		Symbol:  '❌',
		OnClick: func() { wm.RemoveWindow(m) },
	})
	m.AddButton(&winman.Button{
		Symbol: '💾',
		OnClick: func() {
			e = bi.ChangeBucketPolicy(client, policy)
			if e != nil {
				m.SetBorderColor(tcell.ColorRed)
				logger <- fmt.Sprintf("Ошибка смены policy для бакета %s : %s", *bi.Name, e.Error())
			} else {
				m.SetBorderColor(tcell.ColorGreen)
				logger <- fmt.Sprintf("Смена policy для бакета %s выполнено", *bi.Name)
			}
		},
	})
	m.Show()
	app.SetFocus(m)

}
