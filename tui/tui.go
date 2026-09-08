package tui

import (
	"context"

	"github.com/epiclabs-io/winman"
	"github.com/gdamore/tcell/v2"
	"github.com/minio/minio-go/v7"
	"github.com/rivo/tview"
)

func App(minioClient *minio.Client, interactive bool) {

	ctx, cancel := context.WithCancel(context.Background())

	app := tview.NewApplication()   // Инициализация приложения
	wm := winman.NewWindowManager() // Инициализация менеджера окон

	wm.SetBorder(true).SetBorderColor(tcell.Color101)
	wm.SetTitle("S3 FILE MANAGER").SetTitleColor(tcell.Color101)
	wm.Blur()

	footer, logger := makeFoooter(wm, app, minioClient, ctx)
	makeBucketListWindow(wm, minioClient, app, footer, logger, interactive)

	if err := app.SetRoot(wm, true).EnableMouse(true).Run(); err != nil {
		panic(err)
	}
	cancel()
}
