package objectoperations

import (
	"context"
	"fmt"
	"minioclient/utils"
	"sync"
	"sync/atomic"

	"github.com/minio/minio-go/v7"
	"github.com/sirupsen/logrus"
)

// Данный модуль отвечает за операцию миграции данных из бакета
// кластера источника в бакет кластера назначения

// Переменные для формирования отчета
type report struct {
	TotalObjects atomic.Uint64 // Счетчик всех обработанных объектов
	ErrorObjects atomic.Uint64 // Счетчик обработанных объектов с ошибкой
	DoneObjects  atomic.Uint64 // Счетчик обработанных объектов без ошибки
	Errors       []error
}

var finishreport report

// fromclient - клиент кластера источника
// bucketname - имя бакета источника
// toname - название кластера назначения в конфигурации
func MigrateObjects(fromclient *minio.Client, bucketname string, prefix string, toendpoint string, usessl *bool, maxentry int, dstbucketname string) error {

	// Если не указан бакет назначения то бакет назначения делаем равным исходному бакету
	if dstbucketname == "" {
		dstbucketname = bucketname
	}

	// Контекст выполнения операции миграции данных
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Создаем клиента для кластера назначения
	toclient, e := utils.InitClient(&toendpoint, nil, nil, nil, usessl)
	if e != nil {
		logrus.Error(fmt.Sprintf("Error init client for destination cluster %s", e.Error()))
		return e
	}

	objch := make(chan minio.ObjectInfo) // Создаем канал указанного размера для записи объектов продюсеру
	sigch := make(chan bool, maxentry)   // Создаем управляющий канал контроля горутин
	var wg sync.WaitGroup                // Шруппа ожидания завершения всех горути н миграции объектов
	var mu sync.Mutex                    // Мутикс для синхронизации доступа к репорту (добавление ошибок в массив ошибок)
	// Иницируем продюсера
	wg.Add(1)
	go migrateProducer(objch, ctx, bucketname, fromclient, toclient, &wg, sigch, dstbucketname, &mu)

	// Проверяем существует ли бакет в клaстере назначения
	// Если не существует то бакет нужно создать
	found, e := toclient.BucketExists(context.Background(), dstbucketname)
	if e != nil {
		logrus.Error("Error check bucket status on destination! interruption of execution", e.Error())
		return e
	}

	// Если бакет в кластере назначения не найден то пытаемся его создать
	if !found {
		logrus.Info("Bucket ", dstbucketname, " on endpoint ", toclient.EndpointURL(), " does not exist. Creating bucket...")
		e := toclient.MakeBucket(ctx, dstbucketname, minio.MakeBucketOptions{})
		if e != nil {
			// Если не удалось создать бакет то выводим ошибку и завершаем выполнение программы
			logrus.Error("Error create bucket! interruption of execution", e.Error())
			return e
		}
	} else {
		logrus.Info("Bucket ", dstbucketname, " on endpoint ", toclient.EndpointURL(), " already ", "exist.")
	}

	// Получаем список объектов бакета по переданному префиксу
	sourcelist, e := ListBucketDirs(fromclient, &bucketname, nil, &prefix, true, ctx)
	if e != nil {
		logrus.Error(fmt.Sprintf("Error get objects from bucket %s: %s", bucketname, e.Error()))
		return e
	}
	logrus.Info("Init migrate process...")
	// Инициируем операцию миграции данных
	processInit(sourcelist, ctx, toclient, bucketname, fromclient, objch)

	close(objch)        // По окончанию обработки объектов из источника закрываем канал объектов для продюсера миграции
	wg.Wait()           // Дожидаемся окончания всех горутин миграции
	printResultReport() // Выводит результаты миграции
	return nil
}

// Обработчик операции обхода директории бакета
func processInit(sourcelist <-chan minio.ObjectInfo, ctx context.Context,
	toclient *minio.Client, bucketname string, fromclient *minio.Client, objch chan minio.ObjectInfo) {
	// Обработка объектов бакета
	for obj := range sourcelist {
		logrus.Info("Prepaire to migrate object: ", obj.Key)
		select {
		// Если контекст выполнения отменен то прерываем выполнение
		case <-ctx.Done():
			return
		default:
			if obj.Key[len(obj.Key)-1] != '/' {
				// Если это не директория то отдаем объект в продюсера
				// Блокируемая операция зависищая от доступных слотов в канале
				objch <- obj
				// Обработка вложенных директорий бакета (вызываем сами себя)
			} else {
				source, e := ListBucketDirs(fromclient, &bucketname, nil, &obj.Key, true, ctx)
				if e != nil {
					logrus.Error(fmt.Sprintf("Error get objects from bucket %s: %s", bucketname, e.Error()))
					continue
				}
				processInit(source, ctx, toclient, bucketname, fromclient, objch)
			}
		}
	}
}

// Инициатор миграции объекта бакета
func migrateProducer(objch chan minio.ObjectInfo, ctx context.Context, bucketname string,
	fromclient *minio.Client, toclient *minio.Client, wg *sync.WaitGroup, sigch chan bool, dstbucketname string, mu *sync.Mutex) {
	for obj := range objch {
		select {
		case <-ctx.Done():
			return
		default:
			sigch <- true // Занимаем слот управляющего канала
			wg.Add(1)
			go migrateObject(obj.Key, bucketname, fromclient, toclient, ctx, sigch, wg, dstbucketname, mu)
		}
	}
	wg.Done()
}

// Обработчик миграции объекта бакета
func migrateObject(key string, bucketname string, fromclient *minio.Client,
	toclient *minio.Client, ctx context.Context, sigch chan bool, wg *sync.WaitGroup, dstbucketname string, mu *sync.Mutex) {

	finishreport.TotalObjects.Add(1) // Увеличиваем счетчик обработанных обектов
	// Получаем объект из бакета источника
	select {
	case <-ctx.Done():
		return
	default:
		logrus.Info("Migrate object: ", key)
		sourceobj, e := fromclient.GetObject(context.Background(), bucketname, key, minio.GetObjectOptions{
			VersionID: "", // Версионирование не поддерживается мы работаем только с актуальной версией объекта в бакете
			Checksum:  true,
		})
		if e != nil {
			logrus.Error(fmt.Sprintf("Error get source object! Key: %s Error: %s", key, e.Error()))
			finishreport.ErrorObjects.Add(1) // Увеличиваем счетчик обработанных с ошибкой обектов
			mu.Lock()
			finishreport.Errors = append(finishreport.Errors, e) // Сохраняем ошибку
			mu.Unlock()
			return
		}

		// Не забывавем закрыть доступ к исходному объекту
		defer sourceobj.Close()

		// Получаем информацию об объекте (размер)
		stat, e := sourceobj.Stat()
		if e != nil {
			logrus.Error("Error getting object stats: ", e.Error())
			finishreport.ErrorObjects.Add(1) // Увеличиваем счетчик обработанных с ошибкой обектов
			mu.Lock()
			finishreport.Errors = append(finishreport.Errors, e) // Сохраняем ошибку
			mu.Unlock()
			return
		}

		// Записываем объект в бакет назначения
		_, e = toclient.PutObject(ctx, dstbucketname, key, sourceobj, stat.Size,
			minio.PutObjectOptions{ContentType: "application/octet-stream"})
		if e != nil {
			logrus.Error(fmt.Sprintf("Error migrate object: %s Erros: %s", key, e.Error()))
			finishreport.ErrorObjects.Add(1) // Увеличиваем счетчик обработанных с ошибкой обектов
			mu.Lock()
			finishreport.Errors = append(finishreport.Errors, e) // Сохраняем ошибку
			mu.Unlock()
			return
		}
		logrus.Info("Succesfule migrate object: ", key)
		finishreport.DoneObjects.Add(1) // Увеличиваем счетчик успешно обработанных обектов
	}
	<-sigch   // Высвобождаем слот сигнального канала
	wg.Done() // ообщаем об окончании выполнении горутины
}

// Метод печати отчета выполнения операции миграции объектов
func printResultReport() {
	fmt.Printf(`
Migration operation report:
	Total processed objects count: %v
	Migrated objects count: %v
	Failed objects count: %v
`, finishreport.TotalObjects.Load(), finishreport.DoneObjects.Load(), finishreport.ErrorObjects.Load())
	fmt.Println("Errors list:")
	for _, e := range finishreport.Errors {
		fmt.Println(e.Error())
	}
}
