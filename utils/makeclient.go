package utils

import (
	"errors"
	"minioclient/global"
	"os"

	"github.com/ghodss/yaml"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/sirupsen/logrus"
)

// Метод инициализации клиента
func InitClient(endpoint *string, port *string, accessKeyID *string, secretAccessKey *string, useSSL *bool) (*minio.Client, error) {
	// Снача ищем данное подключение в конфигурации
	// Считывание конфигурационного файла
	cfg, e := readcfg()
	// Если воникла ошибка чтения конфигурации тогда
	// Передаваемый endpoint считаем не как имя в конфигурации - а как fqdn узла к которому подключемся
	// СЛОЖНА БЛЯТЬ СЛОЖНА НИ ХУЯ НЕ ПОНЯТНО
	if e != nil {
		logrus.Warn("Error read cfg file: ", e)
		// Если конфигурацию прочитать не удалось то подразумеваем
		// что передан внешний endpoint и нуждны доп аргументы для подключения
		if *port == "" || *endpoint == "" || *secretAccessKey == "" ||
			port == nil || endpoint == nil || secretAccessKey == nil {
			logrus.Error("Please provide port access key id and secret key!")
			os.Exit(1)
		}
		client, e := makeClient(endpoint, port, accessKeyID, secretAccessKey, useSSL)
		if e != nil {
			logrus.Error("Failed to create client!")
			os.Exit(1)
		}
		return client, nil
		// Если конфигурацию нашли и ее удалось прочитать
	} else {
		// Проверяем есть ли переданный endpoint в конфигурации
		for i, connection := range cfg.Connections {
			// Если находим параметры в конфигурации то используем их
			if connection.Name == *endpoint {
				logrus.Info("Found configuration for endpoint: ", *endpoint)
				// Основные парамепптры подключения
				port = &cfg.Connections[i].Port
				accessKeyID = &cfg.Connections[i].Acesskey
				secretAccessKey = &cfg.Connections[i].Secretkey

				// Выбор точки подключения для найденного endpoint
				for y := range cfg.Connections[i].Endpoints {
					// Пробуем подключить к выбранному клиенту
					endpoint = &cfg.Connections[i].Endpoints[y]
					logrus.Info("Selected endpoint: ", *endpoint+":"+*port)
					client, e := makeClient(endpoint, port, accessKeyID, secretAccessKey, useSSL) // Пытаемся установить тестовое подключение
					if e != nil {
						logrus.Error("Error connect to endpoint: ", *endpoint+":"+*port, " Error: ", e)
						if len(cfg.Connections[i].Endpoints)-1 == i {
							logrus.Error("All selected endpoints not available!")
							os.Exit(1)
						}
						continue
					}
					return client, nil
				}
			}
		}
		logrus.Warn("Not found configuration for endpoint: ", *endpoint)
		// Если в конфигурации не нашли нужного подключения то подразумеваем
		// что передан внешний endpoint и нуждны доп аргументы для подключения
		if *port == "" || *accessKeyID == "" || *secretAccessKey == "" ||
			port == nil || endpoint == nil || secretAccessKey == nil {
			logrus.Error("Please provide port access key id and secret key!")
			os.Exit(1)
		}
		client, e := makeClient(endpoint, port, accessKeyID, secretAccessKey, useSSL)
		if e != nil {
			logrus.Error("Failed to create client!")
			os.Exit(1)
		}
		return client, nil
	}
}

// Метод создания подключения к minio серверу
func makeClient(endpoint *string, port *string, accessKeyID *string, secretAccessKey *string, useSSL *bool) (*minio.Client, error) {
	minioClient, e := minio.New(*endpoint+":"+*port, &minio.Options{
		Creds:           credentials.NewStaticV4(*accessKeyID, *secretAccessKey, ""),
		Secure:          *useSSL,
		TrailingHeaders: true,
		MaxRetries:      10,
	})

	if e != nil {
		return nil, e
	}
	logrus.Info("client created")

	b := ConnectionHealthCheck(minioClient)
	if !b {
		logrus.Error("connection health check failed")
		return nil, errors.New("connection health check failed")
	}
	logrus.Info("connection health check passed")
	return minioClient, nil
}

// Метод чтения кофигурационного файла
// Конфигурационный файл должен быть в формате YAML
// Поиск файла осуществляется в следующем порядке:
// 1. Поиск в ./config.yaml
// 2. Поиск в ./config/config.yaml
// 3. Поиск в ./.config/config.yaml
// 4. Если удается определить домашнюю директорию пользователя то в HOMEDIR/.config/config.yaml
func readcfg() (global.Cfg, error) {
	var cfg global.Cfg
	// Директории поиска конфгурационного файла
	dirs := []string{"./", "./config/", "./.config/"}
	h, e := os.UserHomeDir()
	if e != nil {
		dirs = append(dirs, h+"/.config/")
	}

	for _, dir := range dirs {
		_, e = os.Stat(dir + "config.yaml")
		if e != nil {
			continue
		}
		b, e := os.ReadFile(dir + "config.yaml")
		if e != nil {
			continue
		}

		e = yaml.Unmarshal(b, &cfg)
		if e != nil {
			continue
		}

		return cfg, nil
	}

	return cfg, errors.New("configuration file not found")
}
