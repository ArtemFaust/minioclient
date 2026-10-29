package structures

import (
	"strings"

	"github.com/rivo/tview"
)

type ProgressBar struct {
	*tview.List                    // Расширяем существующую структуру пакета tview
	Value       int                // Текучее значение прогресса
	MaxValue    int                // Максимальное значение прогресса
	Text        string             // Поле позволяющее передавать произвольное значение выводимое как доп текст прогресса
	Inf         bool               // Флаг работы infinity метода - false для остановки и сброс бара
	app         *tview.Application // Ссылка на app для принудительного обновления
}

func NewProgressBar(startvalue int, maxvalue int, app *tview.Application) *ProgressBar {
	progress := &ProgressBar{
		List:     tview.NewList(),
		Value:    startvalue,
		MaxValue: maxvalue,
		app:      app,
	}
	progress.ShowSecondaryText(false)
	progress.AddItem(strings.Repeat(" ", progress.Value), progress.Text, ' ', nil)
	return progress
}

// Метод обновления прогресса
func (p *ProgressBar) Update(value int, text string) {
	p.Value = value
	p.Text = text
	p.ShowSecondaryText(true)
	p.Clear()
	p.AddItem(strings.Repeat(" ", p.Value), p.Text, ' ', nil)
	p.app.Draw() // явный вызов обновления (перерисовки)
}

// Метод запуска бесконечного прогресса (зацикленного)
func (p *ProgressBar) Infinity(scoup int, text <-chan string) {
	p.Inf = true
	go progressInfinity(p, scoup, text)
}

// Фоновый метод обновления значений прогресса
func progressInfinity(progress *ProgressBar, scoup int, text <-chan string) {
	lasttext := ""
	for t := range text {
		lasttext = t
		progress.Update(progress.Value+scoup, t)
		if progress.Value >= progress.MaxValue {
			progress.Value = 0
		}
	}
	progress.Value = progress.MaxValue
	progress.Update(progress.Value, lasttext)
}
