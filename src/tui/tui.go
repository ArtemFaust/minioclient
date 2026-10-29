package tui

import (
	"context"
	"minioclient/global"

	"github.com/epiclabs-io/winman"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/sirupsen/logrus"
)

// Глобальное хранение ссылок на окна
var GW global.GlobalWindows

func App(client *global.GlobalClient, interactive bool, usessl *bool, ctx context.Context, region *string) error {

	app := tview.NewApplication()   // Инициализация приложения
	wm := winman.NewWindowManager() // Инициализация менеджера окон

	wm.SetBorder(true).SetBorderColor(tcell.Color101)
	wm.SetTitle("🗂️ S3 FILE MANAGER ").SetTitleColor(tcell.ColorWhite)
	wm.Blur()

	footer, logger := makeFoooter(wm, app, client, ctx, *usessl, &GW, region)
	GW.BucketList = makeBucketListWindow(wm, client, app, footer, logger, interactive, usessl, region)

	if e := app.SetRoot(wm, true).EnableMouse(true).Run(); e != nil {
		logrus.Error("Error init tui interface: ", e.Error())
		return e
	}
	return nil
}
