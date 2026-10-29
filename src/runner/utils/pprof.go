package utils

import (
	"net/http"
	_ "net/http/pprof"

	"github.com/sirupsen/logrus"
)

func Pprof() {
	// Запустим HTTP-сервер для обработки запросов на профилирование
	logrus.Info("Запуск сервиса профилирования на порту *:6060/debug/pprof/")
	go func() {
		e := http.ListenAndServe(":6060", nil)
		if e != nil {
			logrus.Error(e)
		}
	}()
}
