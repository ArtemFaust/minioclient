package tui

import (
	progressbar "minioclient/tui/customwidgets/progressbar"
	"os"

	"github.com/epiclabs-io/winman"
	"github.com/rivo/tview"
	"golang.org/x/term"
)

func progress(wm *winman.Manager, app *tview.Application, title string) (*winman.WindowBase, chan string) {
	// Канал передачи текста в прогресс
	text := make(chan string, 1)
	// Прогресс выполнения операции
	w, h, _ := term.GetSize(int(os.Stdout.Fd()))
	progress_modal := wm.NewWindow()
	progress_modal.SetModal(false)
	progress_modal.SetDraggable(true)
	progress_modal.SetTitle(title)

	progress := progressbar.NewProgressBar(0, 40, app)
	progress.Infinity(1, text)
	progress_modal.SetRect(w/2-25, h/2-2, 50, 4)

	progress_modal.SetRoot(progress)
	progress_modal.Show()
	return progress_modal, text
}
