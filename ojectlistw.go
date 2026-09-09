package tui

import (
	"context"
	"fmt"
	objectoperations "minioclient/object_operations"
	filedialog "minioclient/tui/customwidgets/filedialog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/epiclabs-io/winman"
	"github.com/gdamore/tcell/v2"
	"github.com/minio/minio-go/v7"
	"github.com/rivo/tview"
	"github.com/sirupsen/logrus"
	"golang.org/x/term"
)

// Глобальные статусы

// Блокиратор нужен для того что бы в процессе получения объектов из бакета не вызвали операцию refresh
// так как мы теряем управление над выполняющейся горутиной
// так называемая защита от дребезка (когда пользователь ебашит по кнопке не думая)
var GET_OBJECTS_WORK_STATUS bool

// Метод создания окна BUCKET OBJECT LIST
func makeObjectsListWindow(wm *winman.Manager, minioClient *minio.Client, bucketname string, prefix string,
	title string, app *tview.Application, footer *winman.WindowBase, logger chan string, interactive bool) {

	// Контекст передаваемый в метод получения списка объектов бакета
	ctx, cancel := context.WithCancel(context.Background())

	objectlist := wm.NewWindow()              // Создание нового окна
	objectlist.SetDraggable(true)             // Делаем окно перемечаемым
	objectlist.SetResizable(true)             // Делаем окно маштабируемым
	objectlist.SetTitleColor(tcell.Color101)  // Цвет заголовка
	objectlist.SetBorderColor(tcell.Color101) // Цвет рамки

	if title == "" {
		objectlist.SetTitle(bucketname) // Устанавливаем заголовок
	} else {
		objectlist.SetTitle(title) // Устанавливаем заголовок
	}

	// Добавляем кнопку закрытия окна
	objectlist.AddButton(&winman.Button{
		Symbol: '❌',
		OnClick: func() {
			cancel() // Отменяем контекст выполнения
			wm.RemoveWindow(objectlist)
		},
	})

	// Добавляем кнопку максимализации окна
	objectlist.AddButton(&winman.Button{
		Symbol: '🔼',
		OnClick: func() {
			func() {
				if objectlist.IsMaximized() {
					objectlist.GetButton(1).Symbol = '🔼'
					objectlist.Restore()
				} else {
					objectlist.GetButton(1).Symbol = '🔽'
					objectlist.Maximize()
				}
			}()
		},
	})

	// Добавляем кнопу refresh
	objectlist.AddButton(&winman.Button{
		Symbol: '🔄',
		OnClick: func() {
			// Если уже не выполняется операция получения объектов бакета
			// можно выполнить операцию refresh
			if !GET_OBJECTS_WORK_STATUS {
				objectlist.GetRoot().(*tview.List).Clear()
				objectlist.SetRoot(getObjectList(minioClient, bucketname, prefix, wm, app, footer, logger, ctx, objectlist, interactive))
			}
		},
	})

	// Добавляем кнопку операций
	objectlist.AddButton(&winman.Button{
		Symbol: '🟰',
		OnClick: func() {
			//list := objectlist.GetRoot()
			//key, _ := list.(*tview.List).GetItemText(list.(*tview.List).GetCurrentItem())
			//println(key)

			x, y, _, _ := objectlist.GetRect()
			modal := wm.NewWindow()
			modallist := tview.NewList().
				AddItem("Загрузить", "", '🔼', func() {
					var prefix string
					// Загрузка в корень бакета
					if objectlist.GetTitle() == bucketname {
						prefix = ""
					} else {
						// Загрузка с префиксом пути внутри бакета
						prefix = strings.Split(objectlist.GetTitle(), bucketname)[1][1:] // Префикс формируем из заголовка окна
					}
					logger <- prefix
					fileselectO(wm, app, "Загрузка объектов в бакет", logger, bucketname, minioClient, prefix, interactive)
					modal.Hide()
				}).
				AddItem("Скачать выбранное", "", '🔽', func() {
					main, _ := objectlist.GetRoot().(*tview.List).GetItemText(objectlist.GetRoot().(*tview.List).GetCurrentItem())
					go download(wm, app, minioClient, bucketname, main, "", logger)
					modal.Hide()
				}).
				AddItem("Свойства", "", '❔', func() {
					main, _ := objectlist.GetRoot().(*tview.List).GetItemText(objectlist.GetRoot().(*tview.List).GetCurrentItem())
					getObjectInfo(minioClient, wm, app, bucketname, logger,
						main,
					)
					modal.Hide()
				}).
				AddItem("Список версий", "", '🟰', func() {
					modal.Hide()
				}).
				AddItem("Удалить", "", '➖', func() {
					main, _ := objectlist.GetRoot().(*tview.List).GetItemText(objectlist.GetRoot().(*tview.List).GetCurrentItem())
					go remove(wm, app, minioClient, bucketname, main, "", logger)
					modal.Hide()
				}).
				AddItem("Закрыть", "", '❌', func() { modal.Hide() })
			modal.SetRect(x+10, y, 25, 13)
			modal.SetModal(true)
			modal.SetRoot(modallist)
			app.SetFocus(modal)
			modal.Show()
		},
	})

	objectlist.SetRoot(getObjectList(minioClient, bucketname, prefix, wm, app, footer, logger, ctx, objectlist, interactive)) // Получаем список бакетов
	// rect для текучего окна в фокусе
	x, y, w, _ := app.GetFocus().GetRect()
	_, h, _ := term.GetSize(int(os.Stdout.Fd())) // Высота терминала
	_, _, _, fh := footer.GetRect()              // Высота footer
	objectlist.SetRect(x+w+2, y-1, 30, h-fh-2)   // Свойства окна

	objectlist.Show() // Показываем окно
	app.SetFocus(objectlist)
}

// Метод создания списка объектов
func getObjectList(minioClient *minio.Client, bucketname string, prefix string, wm *winman.Manager,
	app *tview.Application, footer *winman.WindowBase, logger chan string, ctx context.Context,
	objectlist *winman.WindowBase, interactive bool) *tview.List {
	pm, ch := progress(wm, app, "Запрос списка объектов бакета")

	logger <- fmt.Sprintf("Запрос списка объектов бакета %s", bucketname)
	list := tview.NewList()
	list.SetSecondaryTextColor(tcell.ColorSeaGreen)

	// В отдельном потоке набиваем list элеиментами
	go func(pm *winman.WindowBase, list *tview.List, logger chan string, ctx context.Context) {
		GET_OBJECTS_WORK_STATUS = true // Глобально указываем что операция получения объектов из бакета выполняется
		defer func() {
			pm.SetBorderColor(tcell.ColorGreen)
			close(ch)
			time.Sleep(1 * time.Second)
			wm.RemoveWindow(pm)
			GET_OBJECTS_WORK_STATUS = false // Указываем что операция завершилась
		}()

		objectCh, e := objectoperations.ListBucketDirs(minioClient, &bucketname, nil, &prefix, true, ctx)
		if e != nil {
			logrus.Error(e)
			logger <- fmt.Sprintf("Ошибка запрос списка объектов бакета %s: %s", bucketname, e.Error())
			return
		}

		count := 0
		objectlist.SetBorderColor(tcell.ColorYellow)
		for o := range objectCh {
			if ctx.Err() != nil {
				logger <- "Контекст выполнения запроса объектов завершен"
				break
			}
			count++
			ch <- fmt.Sprintf("Полученно объектов: %v", count)
			if strings.HasSuffix(o.Key, "/") {
				key := o.Key
				list.AddItem(o.Key, "", '📁', func() {
					makeObjectsListWindow(wm, minioClient, bucketname, key, bucketname+"/"+key, app, footer, logger, interactive)
				}).SetMainTextColor(tcell.ColorBlue).SetShortcutColor(tcell.NewRGBColor(0, 0, 0))
			} else {
				list.AddItem(o.Key, "", '📄', nil).SetMainTextColor(tcell.ColorYellow).SetShortcutColor(tcell.NewRGBColor(0, 0, 0))
			}
		}
		objectlist.SetBorderColor(tcell.Color101)
		logger <- fmt.Sprintf("Запрос списка объектов бакета %s выполнен", bucketname)
	}(pm, list, logger, ctx)

	return list
}

// Метод выбора файла для загрузки
func fileselectO(wm *winman.Manager, app *tview.Application, title string, logger chan string,
	bucketname string, minioClient *minio.Client, prefix string, interactive bool) {

	m := wm.NewWindow()
	homedir, e := os.UserHomeDir()
	if e != nil {
		homedir = "."
	}

	fd := filedialog.NewFileDialog(homedir, func(filePath string) {
		wm.RemoveWindow(m)
		go func() {
			upload(wm, app, logger, minioClient, filePath, bucketname, prefix, homedir, interactive)
		}()
	})

	m.SetDraggable(true)
	m.SetResizable(true)
	m.SetBorder(true)
	m.SetRect(10, 10, 40, 20)
	m.SetTitle(title)
	m.AddButton(&winman.Button{
		Symbol:  '❌',
		OnClick: func() { wm.RemoveWindow(m) },
	})

	m.AddButton(&winman.Button{
		Symbol: '⏫',
		OnClick: func() {
			go upload(wm, app, logger, minioClient, fd.CurrentSelectedDir, bucketname, prefix, homedir, interactive)
		},
	})

	m.SetRoot(fd)
	m.Show()
	app.SetFocus(m)
}

// Метод загрузки объекта или директории в бакет
func upload(wm *winman.Manager, app *tview.Application, logger chan string,
	minioClient *minio.Client, filePath string, bucketname string, prefix string, basepath string, interactive bool) {
	// Контекст исполнения
	ctx, cancel := context.WithCancel(context.Background())
	// Прогресс выполнения операции
	pm, ch := progress(wm, app, "Загрузка объектов в бакет")
	pm.AddButton(&winman.Button{
		Symbol: '❌',
		OnClick: func() {
			cancel() // отменяем контекст выполнения
			wm.RemoveWindow(pm)
		},
	})
	defer func() {
		close(ch)
		cancel() // отменяем контекст выполнения
	}()
	// Для корректной загрузки директорий нужно избавиться от асолютного пути
	// Для этого нужно сменить рабочую директорию выполнения
	// и обрезать полный путь до локального
	workdir, e := os.Getwd()
	if e != nil {
		workdir = basepath
	}
	os.Chdir(basepath)
	rel, e := filepath.Rel(basepath, filePath)
	if e != nil {
		e = objectoperations.PutBucketObject(minioClient, &filePath, &bucketname, ctx, ch, prefix, interactive)

		if e != nil {
			logger <- fmt.Sprintf("Ошибка загрузки объекта: %s", e.Error())
			pm.SetBorderColor(tcell.ColorRed)
		} else {
			pm.SetBorderColor(tcell.ColorGreen)
			logger <- "Загрузка объета в бакет выполненна"
		}
	} else {

		e = objectoperations.PutBucketObject(minioClient, &rel, &bucketname, ctx, ch, prefix, interactive)

		if e != nil {
			logger <- fmt.Sprintf("Ошибка загрузки объекта: %s", e.Error())
			pm.SetBorderColor(tcell.ColorRed)
		} else {
			pm.SetBorderColor(tcell.ColorGreen)
			logger <- "Загрузка объета в бакет выполненна"
		}
	}
	os.Chdir(workdir)
}

// Метод получения информации об объекте бакета (только конечный объект)
func getObjectInfo(minioClient *minio.Client, wm *winman.Manager, app *tview.Application, bucket_name string,
	logger chan string, objectkey string) {
	w, h, _ := term.GetSize(int(os.Stdout.Fd()))
	m := wm.NewWindow()
	m.SetDraggable(true)
	m.SetResizable(true)
	m.SetBorder(true)
	m.SetRect(w/3, h/4, 70, 40)
	m.AddButton(&winman.Button{
		Symbol: '❌',
		OnClick: func() {
			wm.RemoveWindow(m)
		},
	})

	v := tview.NewTextArea()

	vid := ""
	s, e := objectoperations.GetObjectStat(minioClient, &objectkey, &bucket_name, &vid)
	if e != nil {
		logger <- fmt.Sprintf("Ошибка получения свойств объекта: %s", e.Error())
	}

	v.SetText(s, false)
	m.SetRoot(v)
	m.SetTitle("Свойства объекта " + objectkey)
	m.Show()
	app.SetFocus(m)
}

// Метод выгрузки объекта или директории из бакета на локальную машину
func download(wm *winman.Manager, app *tview.Application, minioClient *minio.Client,
	bucketname string, objectkey string, ver string, logger chan string) {
	// Контекст выполнения
	ctx, cancel := context.WithCancel(context.Background())

	// Прогресс выполнения операции
	pm, ch := progress(wm, app, "Загрузка объектов из бакета: "+bucketname)
	pm.AddButton(&winman.Button{
		Symbol: '❌',
		OnClick: func() {
			cancel() // отменяем контекст выполнения
			wm.RemoveWindow(pm)
		},
	})
	defer func() {
		close(ch) // закрываем канал
		cancel()  // отменяем контекст выполнения
	}()

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

	// Загрузка единичного объекта
	if len(objectkey) > 0 && objectkey[len(objectkey)-1] != '/' {
		downloadSingeObject(minioClient, bucketname, objectkey, ver, logger, ch, ctx)
		pm.SetBorderColor(tcell.ColorGreen) // По окончанию выполнения поля делаем зелеными
		// Загрузка директории целиком
	} else {
		processDirectory(minioClient, bucketname, objectkey, ver, logger, ch, ctx, "download")
		pm.SetBorderColor(tcell.ColorGreen) // По окончанию выполнения поля делаем зелеными
	}

	// Обратная смена рабочей директории
	e = os.Chdir(currentWorkDir)
	if e != nil {
		logger <- fmt.Sprintf("Ошибка смены рабочей директории: %s", e.Error())
		return
	}
}

// Обработчик операции загрузки объекта из бакета
func downloadSingeObject(minioClient *minio.Client, bucketname string,
	objectkey string, ver string, logger chan string, ch chan string, ctx context.Context) {
	select {
	// Если контекст выполнения отменен то прерываем выполнение
	case <-ctx.Done():
		return
	default:
		e := objectoperations.GetObject(minioClient, &objectkey, &bucketname, &ver)
		if e != nil {
			logger <- fmt.Sprintf("Ошибка загрузки объекта: %s", e.Error())
			ch <- fmt.Sprintf("Failed downloaded file: %s ", func(o string) string {
				if len(strings.Split(objectkey, "/")) > 1 {
					return strings.Split(objectkey, "/")[len(strings.Split(objectkey, "/"))-1]
				} else {
					return objectkey
				}
			}(objectkey))
		} else {
			logger <- fmt.Sprintf("Выполненна загрузка объекта: %s", objectkey)
			ch <- fmt.Sprintf("Successfully downloaded file: %s ", func(o string) string {
				if len(strings.Split(objectkey, "/")) > 1 {
					return strings.Split(objectkey, "/")[len(strings.Split(objectkey, "/"))-1]
				} else {
					return objectkey
				}
			}(objectkey))
		}
	}
}

// Метод удаления объектов бакета
func remove(wm *winman.Manager, app *tview.Application, minioClient *minio.Client, bucketname string,
	objectkey string, ver string, logger chan string) {

	w, h, _ := term.GetSize(int(os.Stdout.Fd()))
	m := wm.NewWindow()
	m.SetDraggable(true)
	m.SetBorder(true)
	m.SetRect(w/3, h/4, 30, 5)
	form := tview.NewForm().
		AddButton("Удалить", func() {
			go removeAccept(wm, app, minioClient, bucketname, objectkey, ver, logger)
			wm.RemoveWindow(m)
		}).
		AddButton("Отмена", func() {
			wm.RemoveWindow(m)
		})

	m.SetRoot(form)
	m.SetTitle("Удаление объектов из бакета")
	m.Show()
	app.SetFocus(m)
}

// Метод инициализации процедуры удаления объектов из бакета
func removeAccept(wm *winman.Manager, app *tview.Application, minioClient *minio.Client, bucketname string,
	objectkey string, ver string, logger chan string) {
	// Контекст выполнения
	ctx, cancel := context.WithCancel(context.Background())

	// Прогресс выполнения операции
	pm, ch := progress(wm, app, "Удаление объектов из бакета: "+bucketname)
	pm.AddButton(&winman.Button{
		Symbol: '❌',
		OnClick: func() {
			cancel() // отменяем контекст выполнения
			wm.RemoveWindow(pm)
		},
	})

	defer func() {
		close(ch) // закрываем канал
		cancel()  // отменяем контекст выполнения
	}()

	if len(objectkey) > 0 && objectkey[len(objectkey)-1] != '/' {
		removeSingleObject(minioClient, bucketname, objectkey, ver, logger, ch, ctx)
		pm.SetBorderColor(tcell.ColorGreen) // По окончанию выполнения поля делаем зелеными
	} else {
		processDirectory(minioClient, bucketname, objectkey, ver, logger, ch, ctx, "remove")
		pm.SetBorderColor(tcell.ColorGreen) // По окончанию выполнения поля делаем зелеными
	}
}

// Обработчик удаления объекта из бакета
func removeSingleObject(minioClient *minio.Client, bucketname string,
	objectkey string, ver string, logger chan string, ch chan string, ctx context.Context) {
	select {
	case <-ctx.Done():
		return
	default:
		// Удаляем объект
		force := true
		e := objectoperations.RemoveObject(minioClient, &objectkey, &bucketname, &force, &ver, false)
		if e != nil {
			ch <- fmt.Sprintf("Failed remove file: %s ", func(o string) string {
				if len(strings.Split(objectkey, "/")) > 1 {
					return strings.Split(objectkey, "/")[len(strings.Split(objectkey, "/"))-1]
				} else {
					return objectkey
				}
			}(objectkey))
			logger <- fmt.Sprintf("Ошибка удаления объекта %s: %s", objectkey, e)
			return
		}
		logger <- fmt.Sprintf("Выполненно удаление объекта %s", objectkey)
		ch <- fmt.Sprintf("Successfully remove file: %s ", func(o string) string {
			if len(strings.Split(objectkey, "/")) > 1 {
				return strings.Split(objectkey, "/")[len(strings.Split(objectkey, "/"))-1]
			} else {
				return objectkey
			}
		}(objectkey))
	}
}

// Обработчик операции обхода директории бакета
func processDirectory(minioClient *minio.Client, bucketname string,
	objectkey string, ver string, logger chan string, ch chan string, ctx context.Context, operation string) {
	list, e := objectoperations.ListBucketDirs(minioClient, &bucketname, nil, &objectkey, true, ctx)
	if e != nil {
		logger <- fmt.Sprintf("Ошибка получения списка объектов директории: %s", e.Error())
		return
	}
	// Загрузка всех объектов директории
	for obj := range list {
		select {
		// Если контекст выполнения отменен то прерываем выполнение
		case <-ctx.Done():
			return
		default:
			switch operation {
			case "download":
				// Загрузка объекта
				if obj.Key[len(obj.Key)-1] != '/' {
					// Если это не директория, загрузка объекта
					downloadSingeObject(minioClient, bucketname, obj.Key, ver, logger, ch, ctx)
					// Обработка вложенных директорий бакета (вызываем сами себя)
				} else {
					processDirectory(minioClient, bucketname, obj.Key, ver, logger, ch, ctx, operation)
				}
			case "remove":
				// Удаление объекта
				if obj.Key[len(obj.Key)-1] != '/' {
					// Если это не директория, загрузка объекта
					removeSingleObject(minioClient, bucketname, obj.Key, ver, logger, ch, ctx)
					// Обработка вложенных директорий бакета (вызываем сами себя)
				} else {
					processDirectory(minioClient, bucketname, obj.Key, ver, logger, ch, ctx, operation)
				}
			}
		}
	}
}
