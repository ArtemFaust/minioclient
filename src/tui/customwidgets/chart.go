package customwidgets

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type DataItem struct {
	Label string
	Value int
	Color tcell.Color
}

// Метод создания нового chart смтатистики выполнения миграции
func NewChart(title string, items []DataItem, app *tview.Application, max int) tview.Primitive {
	// Компоненты chart будут распологаться в grid - каждый item на своем row
	grid := tview.NewGrid()
	grid.SetRows(-1)
	grid.SetColumns(-1)
	grid.SetBorders(true)

	// Для каждого элемента расчитываем процент от максимума
	// за максимум сейчас берется total - но по идеи нужно брать ко-во объектов в бакете
	for i, item := range items {
		if max != 0 {
			percentage := (float64(item.Value) / float64(max)) * 100
			if percentage > 100 {
				percentage = 100
			}
			val := strings.Repeat("▒", int(percentage))
			progress := tview.NewTextView().SetText(val).SetTextColor(item.Color)
			grid.AddItem(progress, i, 0, 1, 1, 0, 0, true)
		} else {
			val := "Недостаточно данных"
			progress := tview.NewTextView().SetText(val).SetTextColor(item.Color)
			grid.AddItem(progress, i, 0, 1, 1, 0, 0, true)
		}
	}

	return grid
}
