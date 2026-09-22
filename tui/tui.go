package tui

import (
	"context"
	"minioclient/global"
	"os"

	"github.com/epiclabs-io/winman"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/sirupsen/logrus"
)

func App(client *global.GlobalClient, interactive bool, usessl *bool) {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	app := tview.NewApplication()   // Инициализация приложения
	wm := winman.NewWindowManager() // Инициализация менеджера окон

	wm.SetBorder(true).SetBorderColor(tcell.Color101)
	wm.SetTitle("S3 FILE MANAGER").SetTitleColor(tcell.Color101)
	wm.Blur()

	footer, logger := makeFoooter(wm, app, client, ctx, *usessl)
	makeBucketListWindow(wm, client, app, footer, logger, interactive, usessl)

	if e := app.SetRoot(wm, true).EnableMouse(true).Run(); e != nil {
		logrus.Error("Error init tui interface: ", e.Error())
		os.Exit(1)
	}
}
