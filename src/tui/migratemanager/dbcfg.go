package migratemanager

import (
	"errors"
	"minioclient/global"
	"os"

	"github.com/ghodss/yaml"
)

// Метод чтения кофигурационного файла
// Конфигурационный файл должен быть в формате YAML
// Поиск файла осуществляется в следующем порядке:
// 1. Поиск в ./dbconfig.yaml
// 2. Поиск в ./config/dbconfig.yaml
// 3. Поиск в ./.config/dbconfig.yaml
// 4. Если удается определить домашнюю директорию пользователя то в HOMEDIR/.config/dbconfig.yaml
func ReadDbCfg() (global.DBConfig, string, error) {
	var cfg global.DBConfig
	// Директории поиска конфгурационного файла
	dirs := []string{"./", "./config/", "./.config/"}
	h, e := os.UserHomeDir()
	if e != nil {
		dirs = append(dirs, h+"/.config/")
	}

	for _, dir := range dirs {
		_, e = os.Stat(dir + "dbconfig.yaml")
		if e != nil {
			continue
		}
		b, e := os.ReadFile(dir + "dbconfig.yaml")
		if e != nil {
			continue
		}

		e = yaml.Unmarshal(b, &cfg)
		if e != nil {
			continue
		}

		return cfg, dir + "dbconfig.yaml", nil
	}

	return cfg, "", errors.New("configuration file not found")
}
