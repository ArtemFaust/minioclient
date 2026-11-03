package slave

import (
	"context"
	"fmt"
	"io"
	"log"
	"minioclient/runner/memdb"
	"net/http"
	"sync"
	"time"
	"uuid"

	"github.com/sirupsen/logrus"
)

// Экземпляр in memory kv базы данных
var MDB memdb.MemoryDatabase
var HTTP_PORT string

// Создание http сервиса обработки входячих событий нотификации бакета
// для реализации механизма доливки изменений в бакете за время выполнения мигриации
func NotificationServer(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	logrus.Info("Запуск сервиса http - обработчика нотификаций бакета на порту: " + HTTP_PORT)

	// Создание экземпляра memdb для хранения событий
	_, e := MDB.InitDB()
	if e != nil {
		logrus.Errorf("Ошибка иницализации memdb: %s", e.Error())
		logrus.Error("Работа сервиса приема нотификаций не возможна")
		return
	}

	// Logger ошибок через logrus
	logger := logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:    true,
		TimestampFormat:  "2006-01-02 15:04:05.000",
		ForceColors:      true, // Цвета в терминале
		PadLevelText:     true, // Выравнивание уровней
		QuoteEmptyFields: true, // Кавычки для пустых полей
	})
	w := logger.WriterLevel(logrus.ErrorLevel)
	stdLogger := log.New(w, "", 0)

	defer w.Close()
	// Параметры сервера
	mux := http.NewServeMux()
	server := http.Server{
		Addr:              ":" + HTTP_PORT,  // Порт на котором будет висеть сервис
		ReadTimeout:       10 * time.Second, // Ограничение на чтение
		ReadHeaderTimeout: 10 * time.Second, // Ограничение на чтение заголовков
		WriteTimeout:      10 * time.Second, // Ограничение на запись
		IdleTimeout:       10 * time.Second, // Ограничение на удержание сессии
		MaxHeaderBytes:    4096,             // Ограничение на максимальный размер заголовков
		ErrorLog:          stdLogger,        // Перенаправление лога ошибок в logrus
		Handler:           mux,              // Обработчик через mux
	}
	defer server.Shutdown(ctx)
	defer MDB.DB.Close()

	// Обработчики
	mux.HandleFunc("/", notificationHandler) // Обработчик события нотификации от бакета

	// Старт сервера
	server.ListenAndServe()
}

// Обработчик события нотификации от бакета
func notificationHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)                         // Указываем клиенту, какой метод ожидается
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed) // Отправляем статус 405 (Method Not Allowed) и сообщение

		return
	}
	b, e := io.ReadAll(r.Body)
	if e != nil {
		http.Error(w, "Ошибка чтения тела запроса", http.StatusBadRequest)
		logrus.Errorf("Ошибка чтения тела события нотификации: %s", e.Error())
		return
	}
	logrus.Infof("======= НОВОЕ СОБЫТИЕ ОТ ИСТОЧНИКА %s =======", r.RemoteAddr)
	fmt.Println(string(b))

	key := uuid.NewV4().String()      // генерируем уникальный ключ
	compresed, e := memdb.Compress(b) // Сжимаем значение - событие нотификации

	if e != nil {
		logrus.Errorf("Ошибка выполнения процедуры сжатия значения: %s данные будут сохранены без сжатия", e.Error())
		MDB.CreateOrUpdateRecords(map[string]string{
			key: string(b),
		})
	} else {
		logrus.Infof("Изначальный размер: %d байт Сжатый: %d байт", len(b), len(compresed))
		MDB.CreateOrUpdateRecords(map[string]string{
			key: string(compresed),
		})
	}

	keycount, e := MDB.GetDBKeyCount()
	if e != nil {
		logrus.Errorf("Ошибка получения кол-ва ключей в БД %s", e.Error())
		return
	}
	logrus.Infof("Кол-во ключей в memdb: %d", keycount)
}
