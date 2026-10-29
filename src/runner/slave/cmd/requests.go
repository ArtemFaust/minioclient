package slave

import (
	"context"
	"errors"
	api "minioclient/runner/api"
	utils "minioclient/runner/slave/utils"

	globals "minioclient/runner/slave/globals"
	"runtime"

	"github.com/shirou/gopsutil/v4/mem"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

// Метод отправки статуса воркера мастеру
func ClientHello(masterendpoint string, ctx context.Context, conn *grpc.ClientConn) error {
	select {
	case <-ctx.Done():
		return nil
	default:
		client := api.NewMasterClient(conn)

		// Генерируем UUID
		uuid, hostname, mac, e := utils.GenUuid()
		if e != nil {
			return e
		}

		response, e := client.ClientHello(ctx, &api.ClientHelloRequest{
			Uuid:       uuid,
			HostName:   hostname,
			MacAddress: mac,
			IpAddress:  "", // Будет вычеслен на стороне мастера
			OsType:     runtime.GOOS,
			MemTotal: func() uint64 {
				v, e := mem.VirtualMemory()
				if e != nil {
					return 1
				}
				const gb = 1024 * 1024 * 1024
				return v.Available / gb
			}(),
			CpuCount: int32(runtime.NumCPU()),
			// Данные объявлены глобально мы их только подтягиваем
			Status:     globals.Status.Load(),
			TaskId:     globals.CurrentTaskId.Load(),
			TaskStatus: globals.TaskStatus,
		})

		if response == nil {
			return errors.New("Ошибка responce is nil")
		}

		if response != nil && response.Status != "Allow" {
			logrus.Error("Ошибка подклюбчения к мастеру: ", response.Status, " Детально: ", response.StatusDetailed)
			return errors.New("Ошибка подклюбчения к мастеру: " + response.Status + " Детально: " + response.StatusDetailed)
		}

		logrus.Info("Успешное подключение к мастеру. Статус воркера передан мастеру")

		return nil
	}
}

// Метод запроса задачи от мастера
func GetTask(masterendpoint string, ctx context.Context, conn *grpc.ClientConn) (*api.GetTaskResponse, error) {
	select {
	case <-ctx.Done():
		return nil, nil
	default:
		logrus.Info("Запрос доступных задач")
		client := api.NewMasterClient(conn)

		// Генерируем UUID
		uuid, _, _, e := utils.GenUuid()
		if e != nil {
			return nil, e
		}

		response, e := client.GetTasks(ctx, &api.GetTaskRequest{
			Uuid: uuid,
		})

		if response == nil {
			return nil, errors.New("Ошибка responce is nil")
		}

		// Ошибка со стороны мастера
		if response.Error {
			logrus.Error("Ошибка подклюбчения к мастеру. Детально: ", response.AdditionalMessage)
			return nil, errors.New("Ошибка подклюбчения к мастеру. Детально: " + response.AdditionalMessage)
		}

		// Нет доступных задач
		if response.Id == 0 {
			logrus.Info("Отсутсвуют доступные задачи. Детально: " + response.AdditionalMessage)
			return nil, nil
		}

		// Полученна задача
		if response.Id != 0 {
			// Проверяем что это наша задача
			if response.Uuid != uuid {
				logrus.Warnf("Полученна не своя задача с ID: %s Мой UUID: %s UUID задачи: ", response.Id, uuid, response.Uuid)
				logrus.Warnf("Задача не будет исполнятся так как не предназначенна мне")
				return nil, errors.New("Полученна чужая задача. Исполнение чужих задач не позволенно.")
			}
			logrus.Info("Полученна новая задача с ID: ", response.Id)

			return response, nil
		}
	}
	return nil, nil
}

// Метод запроса обновления статуса задачи миграции
func UpdateMigrateStatus(masterendpoint string, ctx context.Context, conn *grpc.ClientConn) error {
	select {
	case <-ctx.Done():
		return nil
	default:
		client := api.NewMasterClient(conn)

		// Генерируем UUID
		uuid, _, _, e := utils.GenUuid()
		if e != nil {
			return e
		}

		response, e := client.UpdateMigrateTaskStatus(ctx, &api.UpdateMigrateStatusRequest{
			Uuid:   uuid,
			TaskId: globals.CurrentTaskId.Load(),
			TaskStatus: &api.TaskStatus{
				Id:             globals.CurrentTaskId.Load(),
				Status:         globals.TaskStatus,
				StatusDetailed: "",
			},
			MigrateReport: &api.MigrateReport{
				TotalObjects: 0,
				ErrorObjects: 0,
				DoneObjects:  0,
				Errors:       []string{""},
			},
		})

		if response == nil {
			return errors.New("Ошибка responce is nil")
		}

		if !response.UpdateStatus {
			globals.TaskStatus = "error"
			logrus.Error("Ошибка обновления стутуса задачи: ", response.AdditionalMessage)
			return errors.New("Ошибка обновления стутуса задачи: " + response.AdditionalMessage)
		}

		logrus.Info("Статус задачи обновлен: ", response.AdditionalMessage)

		return nil
	}
}
