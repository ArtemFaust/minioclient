package slave

import (
	"bytes"
	"context"
	"errors"
	"net"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

// Метод генерации идентификатора (uuid на основе mac + hostname)
func GenUuid() (string, string, string, error) {
	var guuid string
	hostname, e := os.Hostname()
	if e != nil {
		return guuid, hostname, "", e
	}
	// Получаем mac
	mac, e := getmac()
	if e != nil {
		return guuid, hostname, mac, e
	}
	// Генерируем
	guuid = uuid.NewSHA1(uuid.NameSpaceX500, []byte(mac+hostname)).String()
	return guuid, hostname, mac, nil
}

// Метод получения наибольшего (лексографически) mac адресса
func getmac() (string, error) {
	var largestMac net.HardwareAddr

	interfaces, e := net.Interfaces()
	if e != nil {
		return largestMac.String(), e
	}

	for _, iface := range interfaces {
		// Проверяем, что интерфейс включен (Up) и не является петлевым (Loopback)
		if iface.Flags&net.FlagUp != 0 && iface.Flags&net.FlagLoopback == 0 {
			// Проверяем наличие физического адреса
			if len(iface.HardwareAddr) > 0 {
				// Если это первый найденный адрес или он больше текущего максимального
				if largestMac == nil || bytes.Compare(iface.HardwareAddr, largestMac) > 0 {
					largestMac = iface.HardwareAddr
				}
			}
		}
	}

	// Если нашли хотя бы один подходящий адрес, возвращаем его
	if len(largestMac) > 0 {
		return largestMac.String(), nil
	}

	return "", errors.New("активный MAC-адрес не найден")
}

// Фоновый метод поддержания подключения
func Reconnect(ctx context.Context, conn *grpc.ClientConn) {
	for {
		time.Sleep(1 * time.Second)
		if conn.GetState().String() != "READY" {
			logrus.Warn("Ожидание соединения: STATE - ", conn.GetState().String())
			conn.Connect()
		}
	}
}
