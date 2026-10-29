package slave

import (
	"context"
	cmd "minioclient/runner/slave/cmd"
	"minioclient/runner/utils"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

// Метод запуска slave worker сервиса
func StartWorker(masterendpoint string, ctx context.Context, conn *grpc.ClientConn) {
	logrus.Info("Запуск воркера....")
	var wg sync.WaitGroup
	wg.Add(1)
	// Запуск потока уведомления матсреа о состоянии worker
	go clientHello(masterendpoint, ctx, conn, &wg)
	// Запуск потока получения задач
	wg.Add(1)
	go getTask(masterendpoint, ctx, conn, &wg)
	// Запус к переодической печати отчета нагрузки
	wg.Add(1)
	// Запуск сервиса профилирования
	utils.Pprof()
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		tick := time.Tick(10 * time.Second)
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick:
				utils.PrintStats()
			}
		}
	}(&wg)
	wg.Wait()
}

// Фоновый метод отправки информации о клиенте мастеру
func clientHello(masterendpoint string, ctx context.Context, conn *grpc.ClientConn, wg *sync.WaitGroup) {
	defer wg.Done()
	// Бесконечный цикл отправки состояния worker мастеру
	for {
		time.Sleep(2 * time.Second)
		select {
		case <-ctx.Done():
			return
		default:
			if conn != nil && conn.GetState().String() == "READY" {
				e := cmd.ClientHello(masterendpoint, ctx, conn)
				if e != nil {
					logrus.Error("Ошибка выполнения метода ClientHello - ", e.Error())
				}
			}
		}
	}
}

// Метод инициации запроса задачи и запуска задачи
// если задача поступила
func getTask(masterendpoint string, ctx context.Context, conn *grpc.ClientConn, wg *sync.WaitGroup) {
	defer wg.Done()
	// Бесконечный цикл получения задач от мастера
	for {
		time.Sleep(2 * time.Second)
		select {
		case <-ctx.Done():
			return
		default:
			if conn != nil && conn.GetState().String() == "READY" {
				task, e := cmd.GetTask(masterendpoint, ctx, conn)
				if e != nil {
					logrus.Error("Ошибка выполнения метода GetTask - ", e.Error())
					continue
				}

				// Если задачи нет то пропускаем
				if task == nil {
					continue
				}

				// Если задача полученна то запускаем ее выполнени
				// Блокируючий метод - новые задачи не будут запрашиваться пока не завершится текучая
				e = cmd.RunTask(task, ctx, masterendpoint, conn)
				if e != nil {
					logrus.Error("Ошибка выполнения метода RunTask - ", e.Error())
					continue
				}
			}
		}
	}
}
