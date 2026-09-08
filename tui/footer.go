package tui

import (
	"context"
	"minioclient/utils"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/epiclabs-io/winman"
	"github.com/gdamore/tcell/v2"
	"github.com/minio/minio-go/v7"
	"github.com/rivo/tview"
	"golang.org/x/term"
)

// Метод создания footer
func makeFoooter(wm *winman.Manager, app *tview.Application,
	minioClient *minio.Client, ctx context.Context) (*winman.WindowBase, chan string) {

	logger := make(chan string) // Канал записи логов

	// Footer
	footer := wm.NewWindow()   // Создание нового окна
	footer.SetDraggable(false) // Делаем окно перемечаемым
	footer.SetResizable(false) // Делаем окно маштабируемым
	footer.SetBorder(false)
	footer.SetTitleColor(tcell.Color101)
	footer.SetBorderColor(tcell.Color101) // Цвет рамки
	//footer.SetTitle("")     // Устанавливаем заголовок
	footer.SetModal(false)
	w, h, _ := term.GetSize(int(os.Stdout.Fd()))
	if h/5 < 10 {
		h = 10 * 5
	}
	footer.SetRect(0, h, w, h/5) // Свойства окна
	footer.Show()                // Показываем окно

	// Разметка
	grid := tview.NewGrid().
		SetRows(8).
		SetColumns(-2, -3).
		SetBorders(false)

	// Боксы
	box := wm.NewWindow()
	box.SetTitle("Свойства подключения")
	box.SetBorderColor(tcell.Color101)
	box.SetTitleColor(tcell.Color101)

	box1 := wm.NewWindow()
	box1.SetTitle("Лог операций")
	box1.SetBorderColor(tcell.Color101)
	box1.SetTitleColor(tcell.Color101)

	loglist := tview.NewList() // Логер событий

	box1.SetRoot(loglist)

	grid.AddItem(box, 0, 0, 1, 1, 0, 0, false) // колонка 0 (40%)
	// колонка 1 - пустая (отступ)
	grid.AddItem(box1, 0, 1, 1, 1, 0, 0, false) // колонка 2 (60%)
	footer.SetRoot(grid)

	go updatefooter(footer, logger, ctx)                      // Запускаем в отдельном потоке процесс обновления положения footer
	go addWorkLog(logger, loglist, ctx)                       // Запускаем в отдельном потоке процесс добавления новой записи в лог задач
	go connectionPropertiesUpdate(box, minioClient, app, ctx) // Запускаем в отдельном потоке процесс обновления свойств подключения
	return footer, logger
}

// Метод обновления свойств подключения
func connectionPropertiesUpdate(box *winman.WindowBase,
	minioClient *minio.Client, app *tview.Application, ctx context.Context) {
	l1 := tview.NewTextView().SetText("HOST: ")
	host := tview.NewTextView()
	l2 := tview.NewTextView().SetText("A KEY: ")
	creds := tview.NewTextView()
	l3 := tview.NewTextView().SetText("HEALTH: ")
	health := tview.NewTextView()
	box_grid := tview.NewGrid().
		SetRows(-2, -1, -1, -1).
		SetColumns(0).
		SetBorders(false).
		AddItem(l1, 0, 0, 1, 1, 0, 0, false).
		AddItem(host, 0, 1, 1, 4, 0, 0, false).
		AddItem(l2, 1, 0, 1, 1, 0, 0, false).
		AddItem(creds, 1, 1, 1, 4, 0, 0, false).
		AddItem(l3, 2, 0, 1, 1, 0, 0, false).
		AddItem(health, 2, 1, 1, 4, 0, 0, false)
	box.SetRoot(box_grid)
	for {
		time.Sleep(1 * time.Second)
		select {
		case <-ctx.Done():
			return
		default:
			// Свойства подключения
			host.SetText(func() string {
				url := minioClient.EndpointURL()
				return " " + url.Host
			}())
			creds.SetText(func() string {
				creds, _ := minioClient.GetCreds()
				return " " + creds.AccessKeyID
			}())
			health.SetText(func() string {
				if utils.ConnectionHealthCheck(minioClient) {
					return " " + "✅ OK"
				} else {
					return " " + "❌ ERROR"
				}
			}())
			app.Draw()
		}
		if ctx.Err() != nil {
			return
		}
	}
}

// Метод обновления footer при изменение размера окна
func updatefooter(footer *winman.WindowBase, logger chan string, ctx context.Context) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGWINCH)

	for range sigChan {
		if ctx.Err() != nil {
			break
		}
		w, h, _ := term.GetSize(int(os.Stdout.Fd()))
		if h/5 < 10 {
			h = 10 * 5
		}
		footer.SetRect(0, h, w, h/5)
	}
}

// Метод добавления новой записи в лог задач
func addWorkLog(log <-chan string, loglist *tview.List, ctx context.Context) {
	for l := range log {
		if ctx.Err() != nil {
			break
		}
		if strings.Contains(l, "Ошибка") {
			loglist.AddItem(time.Now().Format("2006-01-02 15:04:05"), l, '🚫', nil).SetSecondaryTextColor(tcell.ColorGray)
			loglist.SetCurrentItem(loglist.GetItemCount() - 1)
		} else {
			loglist.AddItem(time.Now().Format("2006-01-02 15:04:05"), l, '✅', nil).SetSecondaryTextColor(tcell.ColorGray)
			loglist.SetCurrentItem(loglist.GetItemCount() - 1)
		}

	}
}
