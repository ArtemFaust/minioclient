package objectoperations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	bucketoperations "minioclient/bucket_operations"
	"minioclient/global"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/oleiade/reflections"
	"github.com/sirupsen/logrus"
)

// Метод удаления объекта
// если объекта не существует то не вовращет ошибку
// функция завершается успешно
func RemoveObject(minioClient *minio.Client, Key *string, BucketName *string, force *bool, vid *string, dryrun bool) error {
	opts := minio.RemoveObjectOptions{
		GovernanceBypass: true,
		ForceDelete:      *force,
		VersionID:        *vid,
	}
	// Операция удаления выполняется только если dryrun == false
	if !dryrun {
		e := minioClient.RemoveObject(context.Background(), *BucketName, *Key, opts)
		if e != nil {
			logrus.Error(e)
			return e
		}
	}
	logrus.Info("remove object: ", *Key, " versionID: ", opts.VersionID, " succesfule done")
	return nil
}

// Метод массового удаление объектов из бакета через json фаил
func RemoveBucketObjects(minioClient *minio.Client, BucketName *string, force *bool, inputjson string, dryrun bool) error {
	// Проверяем сучествования файла json
	_, e := os.Stat(inputjson)
	if e != nil {
		return e
	}
	// Считываем json
	objects, e := os.ReadFile(inputjson)
	if e != nil {
		return e
	}
	// Формируем массив структур объектов структуры
	//var s3Objects []global.BucketObject
	var s3Objects []minio.ObjectInfo
	e = json.Unmarshal(objects, &s3Objects)
	if e != nil {
		return e
	}

	/*
		// Разбиваем массив на подмассивы
		s3ObjectsChunked := utils.ChunkBy(s3Objects, 100)

		// Инициируем потоки
		lock := make(chan bool, 40) // Канал ограничитель потоков
		var wg sync.WaitGroup       // Группа ожидания потоков
		for _, objects := range s3ObjectsChunked {
			lock <- true                                              // Занимаем слот
			wg.Add(1)                                                 // Увеличиваем счетчик группы потоков
			go c(&objects, lock, &wg, minioClient, BucketName, force) // Запускаем поток
		}
		wg.Wait()   // Ожидаем завершения всех потоков выполнения
		close(lock) // Закрываем канал после завершения всех горутин
	*/

	// Для метода удаления ccmultiple delete
	rch := make(chan minio.ObjectInfo) // Канал передачи оюъектов в метод удаления
	var rwg sync.WaitGroup             // Группа синхронизации для метода удаления ccmultiple
	rwg.Add(1)
	go ccmultiple(minioClient, BucketName, rch, &rwg) // Запуск горутины удаления объектов
	for _, object := range s3Objects {
		logrus.Info("Add object to remove: ", object.Key, " VersionID: ", object.VersionID)
		if !dryrun {
			rch <- object
		}
	}
	close(rch) // По окончанию перебора объектов бакета закрываем канал
	rwg.Wait() // Дожидаемся завершения операций удаления
	return nil
}

// Метод инициализации удаления обхектов из бакета
func c(objects *[]global.BucketObject, lock chan bool, wg *sync.WaitGroup, minioClient *minio.Client, BucketName *string, force *bool, dryrun bool) {
	for _, object := range *objects {
		if e := RemoveObject(minioClient, &object.Name, BucketName, force, &object.VersionID, dryrun); e != nil {
			logrus.Errorf("Failed to remove object %s: %v", object.Name, e)
		}
	}
	<-lock    // Высвобождаем слот
	wg.Done() // Уменьшаем счетчик группы потоков
}

// Метод удаления объектов по ссответствию тегу объекта
func RemoveBucketObjectByTags(minioClient *minio.Client, BucketName *string, force *bool, tag string,
	not bool, dryrun bool, fixleak bool, leakcount int, indexpool string, interactive bool) error {
	// Получаем теги объекта по соотвествиям которым будем проводить удаление
	var o any
	e := json.Unmarshal([]byte(tag), &o)
	if e != nil {
		return fmt.Errorf("invalid JSON format for tag: %v", e)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Проверяем что bucket существует
	f, e := bucketoperations.CheckBucketExist(minioClient, BucketName)
	if e != nil {
		logrus.Error(e)
		return e
	}
	// Если не существует возвращаем ошибку
	if !f {
		e = fmt.Errorf("bucker %s not found", *BucketName)
		logrus.Error(e)
		return e
	}
	// Получаем все объекты из bucket
	logrus.Info("Get bucket objects list...")
	objectCh := minioClient.ListObjects(ctx, *BucketName, minio.ListObjectsOptions{
		Prefix:       "",
		Recursive:    true,
		WithVersions: true,
	})
	// Перебирает обхекты и проводим сравнение переданных значений ключей
	// Если хотя бы одно условие не соответсвует - то переходим к следующему объекту
	//lock := make(chan bool, 40) // Канал используемый для ограничения кол-ва потоков удаления
	//var wg sync.WaitGroup // Группа ожидания завершения горутин

	props, ok := o.(map[string]any)
	if !ok {
		return fmt.Errorf("tag must be a JSON object with string keys")
	}
	switch {
	// Иперация проверки по совпадению значений полей - удалить объекты, которые соответствуют фильтру
	case !not:
		{
			// Для метода удаления ccmultiple delete
			rch := make(chan minio.ObjectInfo, 1000) // Канал передачи оюъектов в метод удаления
			var rwg sync.WaitGroup                   // Группа синхронизации для метода удаления ccmultiple
			rwg.Add(1)
			go ccmultiple(minioClient, BucketName, rch, &rwg) // Запуск горутины удаления объектов
			// Для метода фикса объектов
			fixObjCh := make(chan minio.ObjectInfo, 1000) // Канал передачи оюъектов в метод фикса цикличных объектов
			if fixleak {
				rwg.Add(1)
				go fixleakObjects(BucketName, fixObjCh, &rwg, rch, leakcount, indexpool, minioClient, interactive) // Запуск горутины применения фикса к объектам
			}
			check_objects_count := 0
			skiplist, e := os.ReadFile("./skiplist.txt")
			if e != nil {
				logrus.Warn("Skiplist not fount")
			}
			processed := make(map[string]struct{})

		loop:
			for object := range objectCh {
				if _, exists := processed[object.Key]; exists {
					// Уже обрабатывали этот объект
					continue
				}
				check_objects_count++
				fmt.Printf("\rCheck object count: %d ", check_objects_count)

				// Проверяем наличие ошибок в получении объекта из канала
				if object.Err != nil {
					logrus.Error(object.Err)
					break
				}

				// Сравнение ключа объекта с skiplist
				if skiplist != nil {
					for _, str := range strings.Split(string(skiplist), "\n") {
						if str == object.Key {
							logrus.Infof("Skip object by includig to skiplist: Key: %s Version: %s", object.Key, object.VersionID)
							continue loop
						}
					}
				}

				// Циклом сравниваем значение полей объекта с переданными как аргумент
				for key, value := range props {
					// Получаем значение поля структуры по ключу фильтра переданного как аргумент
					v, e := reflections.GetField(object, key)
					if e != nil {
						logrus.Warn("error get field value: ", e)
						continue loop
					}
					// Если хотя бы одно значение поля не совпадает фильтру то переходим к следующей итерации
					if !reflect.DeepEqual(value, v) {
						continue loop
					}
					/*
						// Если хотя бы одно значение поля не совпадает фильтру то переходим к следующей итерации
						if fmt.Sprintf("%v", value) != fmt.Sprintf("%v", v) {
							continue loop
						}
					*/
				}
				// Иницируем удаление объекта
				//lock <- true
				//wg.Add(1)
				//go cc(object, minioClient, BucketName, force, lock, &wg)
				logrus.Info("Add object to remove: ", object.Key, " VersionID: ", object.VersionID)
				// Операция удаления выполняется только если dryrun == false
				if !dryrun {
					if fixleak {
						fixObjCh <- object
						continue
					}
					rch <- object
				}
				continue loop
			}
			close(rch) // По окончанию перебора объектов бакета закрываем канал
			rwg.Wait() // Дожидаемся завершения операций удаления
		}
	// Операция проверки по не совпадению полей - удалить объекты, которые НЕ соответствуют фильтру
	case not:
		{
			// Для метода удаления ccmultiple delete
			rch := make(chan minio.ObjectInfo, 1000) // Канал передачи оюъектов в метод удаления
			var rwg sync.WaitGroup                   // Группа синхронизации для метода удаления ccmultiple
			rwg.Add(1)
			go ccmultiple(minioClient, BucketName, rch, &rwg) // Запуск горутины удаления объектов
			// Для метода фикса объектов
			fixObjCh := make(chan minio.ObjectInfo, 1000) // Канал передачи оюъектов в метод фикса цикличных объектов
			if fixleak {
				rwg.Add(1)
				go fixleakObjects(BucketName, fixObjCh, &rwg, rch, leakcount, indexpool, minioClient, interactive) // Запуск горутины применения фикса к объектам
			}
			check_objects_count := 0
			skiplist, e := os.ReadFile("./skiplist.txt")
			if e != nil {
				logrus.Warn("Skiplist not fount")
			}

		loop_not:
			for object := range objectCh {
				check_objects_count++
				fmt.Printf("\rCheck object count: %d ", check_objects_count)

				// Проверяем наличие ошибок в получении объекта из канала
				if object.Err != nil {
					logrus.Error(object.Err)
					break
				}

				// Сравнение ключа объекта с skiplist
				if skiplist != nil {
					for _, str := range strings.Split(string(skiplist), "\n") {
						if str == object.Key {
							logrus.Infof("Skip object by includig to skiplist: Key: %s Version: %s", object.Key, object.VersionID)
							continue loop_not
						}
					}
				}

				// Циклом сравниваем значение полей объекта с переданными как аргумент
				for key, value := range props {
					// Получаем значение поля структуры по ключу фильтра переданного как аргумент
					v, e := reflections.GetField(object, key)
					if e != nil {
						logrus.Warn("error get field value: ", e)
						continue loop_not
					}
					// Если хотя бы одно значение поля не совпадает фильтру то иницируем удаление объекта
					if !reflect.DeepEqual(value, v) {
						// Иницируем удаление объекта
						//lock <- true
						//wg.Add(1)
						//go cc(object, minioClient, BucketName, force, lock, &wg)
						logrus.Info("Add object to remove: ", object.Key, " VersionID: ", object.VersionID)
						// Операция удаления выполняется только если dryrun == false
						if !dryrun {
							if fixleak {
								fixObjCh <- object
								continue
							}
							rch <- object
						}
						continue loop_not
					}
					/*
						// Если хотя бы одно значение поля не совпадает фильтру то иницируем удаление объекта
						if fmt.Sprintf("%v", value) != fmt.Sprintf("%v", v) {
							// Иницируем удаление объекта
							lock <- true
							wg.Add(1)
							go cc(object, minioClient, BucketName, force, lock, &wg)
							continue loop_not
						}*/
				}
			}
			close(rch) // По окончанию перебора объектов бакета закрываем канал
			rwg.Wait() // Дожидаемся завершения операций удаления
		}
	}

	//wg.Wait()
	//close(lock)
	return nil
}

/*
// Метод инициализации удаления обхектов из бакета
func cc(object minio.ObjectInfo, minioClient *minio.Client, BucketName *string, force *bool, lock chan bool, wg *sync.WaitGroup) {
	// Иницируем удаление объекта
	e := RemoveObject(minioClient, &object.Key, BucketName, force, &object.VersionID)
	if e != nil {
		logrus.Error("Failed remove object: ", object.Key, " Key: ", object.VersionID)
	}
	wg.Done()
	<-lock
}
*/

// Метод удаления объектов из бакета по LastModified
// Удаляются все объекты с наименьшей LastModified датой
func RmObjectByLastModified(minioClient *minio.Client, BucketName *string, force *bool,
	dryrun bool, fixleak bool, leakcount int, indexpool string, interactive bool) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Для метода удаления ccmultiple delete
	rch := make(chan minio.ObjectInfo, 1000) // Канал передачи оюъектов в метод удаления
	var rwg sync.WaitGroup                   // Группа синхронизации для метода удаления ccmultiple
	rwg.Add(1)
	go ccmultiple(minioClient, BucketName, rch, &rwg) // Запуск горутины удаления объектов

	// Для метода фикса объектов
	fixObjCh := make(chan minio.ObjectInfo, 1000) // Канал передачи оюъектов в метод фикса цикличных объектов
	if fixleak {
		rwg.Add(1)
		go fixleakObjects(BucketName, fixObjCh, &rwg, rch, leakcount, indexpool, minioClient, interactive) // Запуск горутины применения фикса к объектам
	}

	// Проверяем что bucket существует
	f, e := bucketoperations.CheckBucketExist(minioClient, BucketName)
	if e != nil {
		logrus.Error(e)
		return e
	}
	// Если не существует возвращаем ошибку
	if !f {
		e = fmt.Errorf("bucker %s not found", *BucketName)
		logrus.Error(e)
		return e
	}

	// Получаем все объекты из bucket
	logrus.Info("Get bucket objects list...")
	objectCh := minioClient.ListObjects(ctx, *BucketName, minio.ListObjectsOptions{
		Prefix:       "",
		Recursive:    true,
		WithVersions: true,
	})

	check_objects_count := 0
	skiplist, e := os.ReadFile("./skiplist.txt")
	if e != nil {
		logrus.Warn("Skiplist not fount")
	}

	// Буфер хранения объектов - ключ:значение - где:
	// ключ - obkect.Key
	// значение object.LastModified
	objectBuffer := map[string]minio.ObjectInfo{}

	// Начало перебора объектов
	for object := range objectCh {
		check_objects_count++
		fmt.Printf("\rCheck object count: %d ", check_objects_count)

		// Сравнение ключа объекта с skiplist
		if skiplist != nil {
			for _, str := range strings.Split(string(skiplist), "\n") {
				if str == object.Key {
					logrus.Infof("Skip object by includig to skiplist: Key: %s Version: %s", object.Key, object.VersionID)
					continue
				}
			}
		}

		// Проверяем существование ключа в буфере
		// Если объект существует в буфере
		if t, ok := objectBuffer[object.Key]; ok {
			// Если время модификации проверяемого объекта больше объекта в буфере
			if object.LastModified.After(t.LastModified) {
				// Отправляем объект из буфера в канал для удаления
				logrus.Info("Add object to remove: ", object.Key, " VersionID: ", object.VersionID)
				if !dryrun {
					// Если передан флаг фикс объектов
					// то отправляем объект в канал фикса
					if fixleak {
						fixObjCh <- t
						continue
					}
					// Иначе сразу отправляем в канал на удаление
					rch <- t
				}
				// Заменяем объект в буфере на текущий проверяемы
				objectBuffer[object.Key] = object

				// Если время модификации проверяемого объекта меньше объекта в буфере
			} else if t.LastModified.After(object.LastModified) {
				// Отправляем объект из буфера в канал для удаления
				// В буфере объект не меняется
				logrus.Info("Add object to remove: ", object.Key, " VersionID: ", object.VersionID)
				if !dryrun {
					// Если передан флаг фикс объектов
					// то отправляем объект в канал фикса
					if fixleak {
						fixObjCh <- object
						continue
					}
					// Иначе сразу отправляем в канал на удаление
					rch <- object
				}
			}
			// Если объекта с таким ключем нет - то помешаем его в буфер
		} else {
			objectBuffer[object.Key] = object
		}
	}

	time.Sleep(3 * time.Second)
	close(fixObjCh) // По окончанию перебора объектов бакета закрываем канал фикса объектов
	if !fixleak {
		close(rch) // По окончанию перебора объектов бакета закрываем канал удаления объектов
	}
	rwg.Wait() // Дожидаемся завершения операций удаления
	return nil
}

// Фиксер залипших объектов в бакете
// метод получает канал с объектами удаляемыми из бакета
// каждый etag объекта помещается в мапу etag:count
// если count для объекта превысит leakcount то можно считать что объект цикличный и удалить его index из
// гарда индексного пула
func fixleakObjects(BucketName *string, fixObjCh <-chan minio.ObjectInfo, wg *sync.WaitGroup,
	rch chan minio.ObjectInfo, leakcount int, indexpool string, minioClient *minio.Client, interactive bool) {
	// Мапа хранения обрабатываемых объектов
	etagcouter := map[string]*struct {
		Size         int64
		Key          string
		LastModified time.Time
		VersionID    string
		Count        int
		Skip         bool
	}{}
	defer wg.Done()
	for object := range fixObjCh {
		// Проверяем есть ли такой etag в мапе
		// Если etag есть
		if o, ok := etagcouter[object.ETag]; ok {
			// Выполняем проверку соответсвия свойств объекта
			// ДЛЯ ПРОДА ВСЕ ПРОВЕРКИ ДОЛЖНЫ ИСПОЛЬЗОВАТЬСЯ
			if reflect.DeepEqual(object.Size, etagcouter[object.ETag].Size) &&
				reflect.DeepEqual(object.LastModified, etagcouter[object.ETag].LastModified) &&
				reflect.DeepEqual(object.VersionID, etagcouter[object.ETag].VersionID) &&
				reflect.DeepEqual(object.Key, etagcouter[object.ETag].Key) {
				o.Count++ // Увеличиваем сетчик объекта на 1
			} else {
				logrus.Warn("Атрибуты объекта не совпадают")
				continue
			}
			// Если счетчик объекта достиг предела то данный обхект считается циклично залипшим
			if o.Count >= leakcount {
				// Если оюъект уже обработан то пропускаем его
				if o.Skip {
					continue
				}
				// Реализация удаления индекса связанного с объектом rgw
				logrus.Warn("Detect leack object: Etag: ", object.ETag, " Key: ", object.Key, " IsLatest: ", object.IsLatest)
				e := deleteObjectIndexFromIndexPool(*BucketName, object, indexpool, minioClient, interactive)
				if e != nil {
					logrus.Error("Error delete object form index pool: ", e)
				} else {
					logrus.Info("Succes delete object from object pool")
				}
				// После обрабротки оюъекта (удаления индексных записей) устанавливаем атрибут Skip для последующих проверок
				// Так как оюъект может повторяться
				o.Skip = true
				continue
			} else {
				if rch != nil {
					rch <- object
				}
			}
			// Если такого объекта нет в мапе то добавляем его
		} else {
			// Создаем новый объект в мапе
			etagcouter[object.ETag] = &struct {
				Size         int64
				Key          string
				LastModified time.Time
				VersionID    string
				Count        int
				Skip         bool
			}{
				Size: object.Size, Key: object.Key, LastModified: object.LastModified, VersionID: object.VersionID, Count: 1, Skip: false,
			}
			// Отправляем объект в канал на удаление
			if rch != nil {
				rch <- object
			}
		}
	}
	// Закрываем канал здесь
	close(rch)
}

// Метод удаления объекта из индексного пула
func deleteObjectIndexFromIndexPool(bucket string, object minio.ObjectInfo, indexpoolname string, minioClient *minio.Client, interactive bool) error {
	// Получаем элементы индекса объекта
	objectindex, e := getObjectIndexEntryes(bucket, object.Key)
	if e != nil {
		logrus.Error(e)
		return e
	}
	logrus.Info("For object: ", object.Key, " found index entryes count: ", len(objectindex.([]any)))

	// Получаем статистику бакета (его свойства)
	bucketstats, e := getBucketStats(bucket)
	if e != nil {
		logrus.Error(e)
		return e
	}
	logrus.Info("Bucket id: ", bucketstats.ID)

	// Получаем массив шардов содержачих относячийся к нашему бакету
	currect_shards, e := getBucketShards(indexpoolname, bucketstats)
	if e != nil {
		logrus.Error(e)
		return e
	}
	logrus.Info("Shards count: ", len(currect_shards))

	// олучаем мапу shard_name:[keys]
	object_shard_keys := objectIndexKeyFinder(currect_shards, indexpoolname)

	// Cкачиваем объект в папку бекапа - бекап объекта перед удалением
	os.Mkdir("backup", 0644)
	os.Chdir("./backup")
	e = GetObject(minioClient, &object.Key, &bucket, &object.VersionID)
	if e != nil {
		logrus.Error("Failed create backup file!", e)
	} else {
		logrus.Info("Create buckup file succesfule!")
	}
	os.Chdir("../")

	// Получаем байтовое представление всех индексных записей шардов
	// и производим их удаление
	for shard, data := range object_shard_keys {
		result := [][]byte{}
		if len(data) == 0 {
			continue
		}
		// Сплитик байты по символу '\n'
		start := 0
		for i, b := range data {
			if b == '\n' {
				// Проверяем что имя индекса содержит имя объекта
				if !strings.Contains(string(data[start:i]), object.Key) {
					start = i + 1
					continue
				}
				result = append(result, data[start:i])
				start = i + 1
			}
		}
		if start < len(data) {
			result = append(result, data[start:])
		}

		// Удаляем записи из индекса
		for i, d := range result {
			// Если ключ индекса не содержит имя объекта то пропускаем его
			if !strings.Contains(string(d), object.Key) {
				continue
			}

			//Тут мы реализуем поиск именно OLH ключа объекта
			var olh global.OlhObjectIndexRecord
			for _, index := range objectindex.([]any) {
				// Сначала маршалим объект any
				b, e := json.Marshal(index)
				if e != nil {
					return e
				}
				// Пытаемся преобразовать объект в структуру OLH
				e = json.Unmarshal(b, &olh)
				if e != nil {
					continue // Если не удалось преобразовать переходим к слудеющему объекту индексной записи
				} else {
					break // Если удалось то прерываем цикл
				}
			}

			// Нас интерисует IDX OLH записи
			// Это объект содержащий все версии instance
			logrus.Info("OLH record: ", olh.Idx, " for object: ", object.Key)
			_, e := getIndexValue(i, d, indexpoolname, shard)
			if e != nil {
				logrus.Error("Error get omapval for object!")
			}

			/*// Пытаемся определить соответсвует ли dump значения ключа OLH записи
			if strings.Contains(string(o), "00000000  01 01") {
				// Удаляем индекс
				e = removeKeyFromShardIndexPool(i, d, indexpoolname, shard)
				if e != nil {
					logrus.Error("Error remove index record: ", e)
				}
			}*/

			e = removeKeyFromShardIndexPool(i, d, indexpoolname, shard)
			if e != nil {
				logrus.Error("Error remove index record: ", e)
			}
		}
	}
	// Востанавливаем файлы из бекапа
	os.Chdir("backup")
	entries, e := os.ReadDir("./")
	if e != nil {
		logrus.Error("Failed read backup dir!")
		return nil
	}
	for _, entry := range entries {
		bname := entry.Name()
		// Загрузка объекта в бакет
		// Загружаем объект полный путь которого соотвествует object.Key
		if s, e := filepath.Abs("./" + object.Key); e != nil {
			if s == object.Key {
				ctx := context.Background()
				e = PutBucketObject(minioClient, &bname, &bucket, ctx, nil, "", interactive)
				if e != nil {
					logrus.Error("Failed restore object from backup!")
				}
			}
		}
	}
	os.Chdir("../")
	return nil
}

// Метод поиска ключей индекса объекта в шардах
func objectIndexKeyFinder(currect_shards []string, indexpoolname string) map[string][]byte {
	// азбтваем массив на подмассивы - будем запукать 10 потоков выполнения перебора индексов щарда
	var wg sync.WaitGroup                    // Группа синхронизации потоков
	var mu sync.Mutex                        // Обеспечение эксклюзивного доступа к общим данным из горутин
	object_shard_keys := map[string][]byte{} // Результирующая мапа хранения shard_name:[keys..]
	// Запускаем горутины поиска ключей в шардах
	for chunked_slice_shards := range slices.Chunk(currect_shards, len(currect_shards)/10) {
		wg.Add(1)
		go func(shards []string, wg *sync.WaitGroup, mu *sync.Mutex) {
			defer wg.Done()
			// Ищем ключи в шарде
			for _, shard := range shards {
				args := []string{"-p", indexpoolname, "listomapkeys", shard, "| cat -A"}
				b, e := exec.Command("rados", args...).Output()
				if e != nil {
					logrus.Error("Failed find keys from shard: ", shard)
					continue
				}
				if len(b) > 0 {
					mu.Lock()
					object_shard_keys[shard] = b
					mu.Unlock()
				}
			}
		}(chunked_slice_shards, &wg, &mu)
	}
	wg.Wait() // Ожидаем завершения всех горутин
	return object_shard_keys
}

// Метод получения шардов бакета
func getBucketShards(indexpoolname string, bucketstats global.BucketStats) ([]string, error) {
	// Получаем список шардов бакета в индексном пуле
	currect_shards := []string{}
	logrus.Info("Get bucket index shards for index pool: ", indexpoolname)
	args := []string{"-p", indexpoolname, "ls"}

	b, e := exec.Command("rados", args...).Output()
	if e != nil {
		fmt.Println(string(b))
		return currect_shards, e
	}
	if len(b) == 0 {
		return currect_shards, nil
	}
	shards := strings.Split(string(b), "\n")

	if len(shards) == 0 {
		return currect_shards, nil
	}
	// Удаляем лишние шарды из списка - не содержащие ID бакета
	for _, shard := range shards {
		if strings.Contains(shard, bucketstats.ID) {
			currect_shards = append(currect_shards, shard)
		}
	}

	return currect_shards, nil
}

// Метод получения stats бакета
func getBucketStats(bucket string) (global.BucketStats, error) {
	// Получаем id бакета
	var bucketstats global.BucketStats
	args := []string{"bucket", "stats", "--bucket", bucket}
	b, e := exec.Command("radosgw-admin", args...).Output()
	if e != nil {
		return bucketstats, e
	}
	e = json.Unmarshal(b, &bucketstats)
	if e != nil {
		return bucketstats, e
	}
	return bucketstats, nil
}

// Метод получения список индексных записей объекта бакета
func getObjectIndexEntryes(bucket string, key string) (any, error) {
	// Получаем список индексных записей объекта бакета
	var objectindex any
	args := []string{"bi", "list", "--bucket", bucket, "--object", key}
	b, e := exec.Command("radosgw-admin", args...).Output()
	if e != nil {
		return objectindex, e
	}
	e = json.Unmarshal(b, &objectindex)
	if e != nil {
		return objectindex, e
	}
	// Проверяем что мы дествительно получили массив объектов а не какую то залупу
	if _, ok := objectindex.([]any); !ok {
		return objectindex, errors.New("Internal error ...")
	}
	// Если массив индекса пуст то это и не ошибка просто возвращаем nil
	if len(objectindex.([]any)) == 0 {
		return objectindex, errors.New("index entryes not found")
	}
	return objectindex, nil
}

// Метод удаления индексной запсии
func removeKeyFromShardIndexPool(i int, d []byte, indexpoolname string, shard string) error {
	// Удаляем индекс
	e := os.WriteFile(strconv.Itoa(i)+".bin", d, 0644)
	if e != nil {
		return e
	}
	// Nтут реализуется удаление объекта из индекса
	logrus.Info("Remove key from index pool: ", indexpoolname, " shard: ", shard, " key: ", string(d))
	args := []string{"-p", indexpoolname, "rmomapkey", shard, "--omap-key-file", strconv.Itoa(i) + ".bin"}
	_, e = exec.Command("rados", args...).Output()
	if e != nil {
		return e
	}
	logrus.Info("Remove keys from index pool")
	e = os.Remove(strconv.Itoa(i) + ".bin")
	if e != nil {
		return e
	}
	return nil
}

// Метод получения значения ключа
func getIndexValue(i int, d []byte, indexpoolname string, shard string) ([]byte, error) {
	// Удаляем индекс
	e := os.WriteFile(strconv.Itoa(i)+".bin", d, 0644)
	if e != nil {
		return nil, e
	}

	args := []string{"-p", indexpoolname, "getomapval", shard, "--omap-key-file", strconv.Itoa(i) + ".bin"}
	o, e := exec.Command("rados", args...).Output()
	if e != nil {
		return nil, e
	}

	e = os.Remove(strconv.Itoa(i) + ".bin")
	if e != nil {
		return nil, e
	}
	return o, nil
}

// Для массового удаления можно воспользоваться данным механизмом удаления объектов из бакета
func ccmultiple(minioClient *minio.Client, BucketName *string, objectsCh <-chan minio.ObjectInfo, wg *sync.WaitGroup) {
	ctx, cancel := context.WithCancel(context.Background())
	defer wg.Done()
	defer cancel()

	// Пока в каналде есть объекты передаем их методу api для массового удаления
	for e := range minioClient.RemoveObjects(ctx, *BucketName, objectsCh, minio.RemoveObjectsOptions{GovernanceBypass: true}) {
		if e.Err != nil {
			logrus.Error("Failed remove object: ", e.ObjectName, "VersionID: ", e.VersionID, " Err: ", e.Err)
		}
	}
	logrus.Info("Success remove objects: done")
}
