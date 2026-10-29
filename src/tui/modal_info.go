package tui

import (
	"os"

	"github.com/epiclabs-io/winman"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"golang.org/x/term"
)

func makeInfoModal(wm *winman.Manager, app *tview.Application, text string) {
	modal := wm.NewWindow()
	// Модальное окно
	m := tview.NewModal()
	w, h, _ := term.GetSize(int(os.Stdout.Fd()))
	//m.SetTitle("Внимание")
	m.SetText(text)
	m.SetBorder(false)
	m.SetBackgroundColor(tcell.ColorGrey)

	m.AddButtons([]string{"Закрыть"})
	m.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		if buttonLabel == "Закрыть" {
			wm.RemoveWindow(modal)
		}
	})

	modal.SetDraggable(false)
	modal.SetBorder(false)
	modal.SetRect(0, 0, w, h)

	modal.SetRoot(m)
	modal.Show()
	app.SetFocus(modal)

}
