package structures

import (
	"os"
	"path/filepath"

	"github.com/rivo/tview"
)

// Кастомный диалог выбора файла
type FileDialog struct {
	*tview.List                     // Настледуем структуру tlist
	rootDir            string       // Стартовая диреткория
	onSelect           func(string) // Функиц явыполняющаяся при выборе файла
	CurrentSelectedDir string       // Текучий выбранный элемент директория
}

// Инициализация структуры
func NewFileDialog(startDir string, onSelect func(string)) *FileDialog {
	fd := &FileDialog{
		List:     tview.NewList(),
		rootDir:  startDir,
		onSelect: onSelect,
	}
	fd.updateList()
	return fd
}

func (fd *FileDialog) updateList() {
	fd.Clear()

	fd.ShowSecondaryText(false)

	fd.AddItem("..", "", 0, func() {
		fd.rootDir = filepath.Dir(fd.rootDir)
		fd.updateList()
	})

	files, e := os.ReadDir(fd.rootDir)
	if e != nil {
		fd.AddItem("Ошибка чтения директории", e.Error(), 0, nil)
		return
	}

	for _, file := range files {
		name := file.Name()
		if file.IsDir() {
			name += "/"
		}

		path := filepath.Join(fd.rootDir, file.Name())
		isDir := file.IsDir()

		if isDir {
			fd.AddItem(name, "", '📁', func() {
				// Необходимый костыль для реализации двойного клика по директории
				if fd.CurrentSelectedDir == path {
					fd.rootDir = path
					fd.updateList()
				} else {
					fd.CurrentSelectedDir = path
				}
			})
		} else {
			fd.AddItem(name, "", '📄', func() {
				fd.onSelect(path)
			})
		}
	}
}
