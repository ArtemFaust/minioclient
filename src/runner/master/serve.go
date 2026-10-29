package master

import (
	"context"
	"minioclient/global"
	api "minioclient/runner/api"
	"minioclient/runner/master/cmd"
	"minioclient/runner/utils"
	"net"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"gorm.io/gorm"
)

// Запуск RPC сервера на master
func MasterServe(port string, d *gorm.DB, dbcfg *global.DBConfig) {
	server := grpc.NewServer()
	master := cmd.Master{
		Db:    d,
		Dbcfg: dbcfg,
	}
	api.RegisterMasterServer(server, &master)

	// Создаем новый listener
	listen, e := net.Listen("tcp", ":8080")
	if e != nil {
		logrus.Fatal("Ошибка запуска rpc сервиса: ", e.Error())
	}

	// Запускаем поток выполнения проверки доступности workers
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go cmd.HealthCheckWorkers(ctx, master.Db) // Запуск фонового процесса проверки доступности workers
	go cmd.AutoWorkerChanger(ctx, master.Db)  // Запуск фонового процесса переназначения задач с недоступных workers
	// Запус к переодической печати отчета нагрузки
	go func() {
		tick := time.Tick(10 * time.Second)
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick:
				utils.PrintStats()
			}
		}
	}()
	// Запуск сервиса профилирования
	utils.Pprof()
	// Запускаем наш сервер на указанном listener
	logrus.Info("Запуск RPC сервиса master управления воркерами. Port: ", port)
	e = server.Serve(listen)
	if e != nil {
		logrus.Fatal("Ошибка запуска rpc сервиса: ", e.Error())
	}
}
