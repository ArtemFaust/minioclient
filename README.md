# minioclient

CLI клиент для управления объектным S3 совместимым хранилищем с поддержкой TUI интерфейса.

## Особенности

- Управление bucket'ами (создание, удаление, проверка существования, список со статистикой)
- Работа с объектами (загрузка, удаление, обновление, получение метаданных)
- Просмотр директоров в стиле UNIX
- Поддержка версионирования объектов
- TUI консоль с интерактивным управлением
- Массовое удаление объектов по JSON файлу или тегам
- Режим dry-run для безопасного тестирования
- Работает с S3 API и Ceph RGW

## Требования

- Go 1.21+
- MinIO сервер, Ceph RGW или совместимое S3 хранилище

## Установка

```bash
go build -o minioclient .
```

## Использование

### Быстрый старт

#### 1. Создайте конфигурационный файл `config.yaml`:

```yaml
connections:
  - name: local_minio
    port: "9000"
    accesskey: "minioadmin"
    secretkey: "minioadmin"
    ssl: false
    endpoints:
      - localhost:9000
      - 127.0.0.1:9000
```

#### 2. Основные команды и их флаги:

**Создать bucket:**
```bash
./minioclient -e <connection_name> -mb -bn <bucket_name> [-ol]
```
- `-mb` – создать новый bucket
- `-bn` – имя bucket
- `-ol` – включить object locking
- `-r, --region` – регион (по умолчанию us-east-1)

**Удалить bucket:**
```bash
./minioclient -e <connection_name> -db -bn <bucket_name>
```
- `-db` – удалить существующий bucket (bucket должен быть пуст)

**Список bucket'ов:**
```bash
./minioclient -e <connection_name> -lb [-o table/json] [-prefix <prefix>] [-maxentry <count>] [-maxthreads <count>] [-endpoint <host>] [-port <port>]
```
- `-lb` – список всех bucket'ов
- `-o` – формат вывода (json/table, по умолчанию json)
- `-prefix` – фильтр по префиксу
- `-maxentry` – лимит на запись (по умолчанию 1000)
- `-maxentry, --maxthreads` – количество потоков для операции миграции (по умолчанию 1000)

**Проверить существование bucket:**
```bash
./minioclient -e <connection_name> -bn <bucket_name> [-o table/json]
```

**Получить информацию о bucket:**
```bash
./minioclient -e <connection_name> -bn <bucket_name> [-o table/json]
```

**Загрузить объект (файл или директорию):**
```bash
./minioclient -e <connection_name> -po -bn <bucket_name> -path <file_path> [-f]
```
- `-po` – загрузить новый объект (если существует, создаёт новую версию)
- `-path` – путь к файлу или директории для загрузки
- `-f` – force режим

**Обновить существующий объект:**
```bash
./minioclient -e <connection_name> -ao -bn <bucket_name> -path <file_path>
```
- `-ao` – append (обновить существующий объект)

**Удалить конкретный объект:**
```bash
./minioclient -e <connection_name> -ro -bn <bucket_name> -key <object_key> [-f] [-vid <version_id>] [-dryrun]
```
- `-ro` – удалить объект по ключу
- `-key` – ключ объекта
- `-f` – force удаление (включая delete markers)
- `-vid` – версию для удаления (если объекм имеет версионирование)
- `-dryrun` – не применять изменения

**Массовое удаление объектов из JSON файла:**
```bash
./minioclient -e <connection_name> -ro -bn <bucket_name> -path <json_file> [-f] [-dryrun]
```

**Удаление по тегам (эквиваленты всех условий):**
```bash
./minioclient -e <connection_name> -ro -bn <bucket_name> -tags '{"tag1":"val1","tag2":"val2"}' [-not] [-f] [-dryrun]
```
- `-tags` – JSON с теговыми условиями удаления
- `-not` – инвертировать логику (удалять, если условия НЕ совпадают)

**Удаление старых версий неактуальных объектов:**
```bash
./minioclient -e <connection_name> -ro -bn <bucket_name> -bylastmodify [-fixleak] [-leakcount <count>] [-indexpool <pool>] [-dryrun]
```
- `-bylastmodify` – удалить объекты по last-modify (оставить только latest)
- `-fixleak` – исправить утечки в RGW индексе
- `-leakcount` – порог срабатывания (по умолчанию 10)
- `-indexpool` – имя index pool (default.rgw.buckets.index)

**Список объектов в bucket:**
```bash
./minioclient -e <connection_name> -lbo -bn <bucket_name> [-o table/json] [-sv] [-prefix <prefix>] [-maxentry <count>] [-ls]
```
- `-lbo` – список объектов в bucket
- `-sv` – показывать версии файлов
- `-prefix` – фильтр по префиксу объекта
- `-maxentry` – лимит записей (по умолчанию 1000)
- `-ls` – Unix стиль списка (директории и файлы с индикацией)

**Список незавершённых загрузок:**
```bash
./minioclient -e <connection_name> -liu -bn <bucket_name> [-o table/json]
```
- `liu` – list incomplete uploads

**Получить метаданные объекта:**
```bash
./minioclient -e <connection_name> -gos -bn <bucket_name> -key <object_key> [-vid <version_id>]
```
- `-gos` – get object stats (только JSON)
- `-vid` – версия объекта

**Получить файл объекта:**
```bash
./minioclient -e <connection_name> -go -bn <bucket_name> -key <object_key> [-vid <version_id>]
```
- `-go` – get object (вывод в JSON формате)

**Запустить TUI консоль:**
```bash
./minioclient -i -e <connection_name>
```

## Настройка

Файл конфигурации `config.yaml` определяет подключения. Поддерживается несколько endpoints для повышения доступности.

Имена подключений из конфига могут использоваться вместо `-e <fqdn>`.

### Путь к конфигурации

Исследуется в следующем порядке:
1. `./config.yaml`
2. `./config/config.yaml`
3. `./.config/config.yaml`
4. `~/.config/config.yaml`

## Аргументы командной строки

| Флаг | Описание |
|------|----------|
| `-examples, --examples` | Показать примеры использования |
| `-e, --endpoint <name>` | Имя подключения из конфига или FQDN (обязательный атрибут) |
| `-port <port>` | API порт (используется из конфига если не задан явно) |
| `-accesskey <key>` | Access key ID |
| `-secretkey <key>` | Secret access key |
| `-ssl [true\|false]` | Использовать HTTPS (по умолчанию true, задать -ssl=false для HTTP) |
| `-mb` | Создать новый bucket |
| `-bn <bucket_name>` | Название bucket |
| `-r, --region <region>` | Регион (по умолчанию us-east-1) |
| `-db` | Удалить существующий bucket (bucket должен быть пуст) |
| `-lb` | Список всех bucket'ов |
| `-lbo <bucket_name>` | Список объектов в конкретном bucket |
| `-ls` | Unix стиль списка бакета (директории и файлы) |
| `-liu <bucket_name>` | Список незавершённых загрузок |
| `-po -bn <buket> -path <file>` | Загрузить новый объект (файл или директорию) |
| `-ao -bn <buket> -path <file>` | Обновить существующий объект (append) |
| `-ro -bn <bucket> -key <obj>` | Удалить объект по ключу |
| `-tags '<json>'` | JSON условия удаления по тегам |
| `-not` | Инвертировать логику работы с тегами |
| `-dryrun` | Не применять изменения (dry-run) |
| `-path <file>` | Путь к файлу для загрузки (массовое удаление) |
| `-vid <version_id>` | Version ID для целевого объекта (при операции удаления/получения) |
| `-bylastmodify` | Удаление объектов кроме последнего по last-modify времени |
| `-sv` | Показывать версии файлов в списке |
| `-prefix <prefix>` | Фильтр по префиксу при работе с объектами |
| `-maxentry <count>` | Максимальное кол-во записей при списке (по умолчанию 1000) |
| `-f, --force` | Force режим операций |
| `-o, --output[=json\|table]` | Формат вывода: json или table (по умолчанию json) |
| `-d, --debug` | Включить вывод debug сообщений |
| `-i, --interactive` | Запустить TUI интерфейс |
| `-ol` | Enable object locking для bucket |
| `-fixleak` | Исправить утечки в RGW индексе (для -ro) |
| `-leakcount <count>` | Порог срабатывания для fixleak (по умолчанию 10) |
| `-indexpool <pool>` | Имя index pool для RGW (по умолчанию default.rgw.buckets.index) |
| `-tpo <count>` | Макс объектов на таблице для табличного принтера (по умолчанию 20) |
| `-dbn <destination_bucket>` | Имя бакета назначения при миграции объектов. Если не указано то имя бакета назначения будет равно имени бакета источника. |
| `-migrate` | Инициализация операции миграции объектов бакета |
| `-destination <cluster_from_cfg>` | Название кластера куда будет выполнятся миграция объектов бакета |
| `-maxthreads, --maxentry <count>` | Количество потоков для операции миграции (по умолчанию 1000) |

## Примеры конфигурации

```yaml
connections:
  - name: localdev
    port: "9000"
    accesskey: "minioadmin"
    secretkey: "minioadmin"
    ssl: false
    endpoints:
      - localhost:9000
      - 127.0.0.1:9000

  - name: production_minio
    port: "9000"
    accesskey: "${MINIO_ACCESS_KEY}"
    secretkey: "${MINIO_SECRET_KEY}"
    ssl: true
    endpoints:
      - api.minio.example.com
      - backup.minio.example.com

  - name: cephrgw
    port: "8080"
    accesskey: "rgw-admin"
    secretkey: "${RGW_SECRET_KEY}"
    ssl: false
    usepath: true
    endpoint: ceph.example.com/srv/ceph/object
```

## Примеры выполнения

### Созданиеbucket с locking support:
```bash
./minioclient -e localdev -mb -bn mybackup -ol
```

### Список Buckley со статистикойв таблице:
```bash
./minioclient -e localdev -lb -o table
```

### Загрузка директории с force режимом: 
```bash
./minioclient -e localdev -po -bn mybackup -path ./docs -f
```

### Просмотр объектов в Unix стиле:
```bash
./minioclient -e localdev -lbo -bn mybackup -o table -ls -sv
```

### Массовое удаление неактуальных версий с исправлением утечек:
```bash
./minioclient -e cephrgw -ro -bn archives -bylastmodify -fixleak -leakcount 10 -d
```

### Миграция бакета из одного кластера в другой или в рамках одного кластера но в другой бакет:
```bash
./minioclient -e <source cluster from cfg> -ssl=false -migrate -destination <dst cluster from cfg> -bn <source bucket> [-dbn <dst bucket>] [-d] [-maxentry <threads count>]
```
Данная операция позволяет произвести миграцию объектов бакета в другой кластер или в тот же клстер но в другой бакет. Если не указан destination bucket - значит будет создан бакет с названием source bucket Каждый объект бакета обрабатывается в отдельном потоке исполнения операции миграции. Ключем maxentry можно указать сколько таких потов будет выполнятся одновременно (сколько объектов бакета будет мигрировать одновременно). По умолчанию 1000

Дополнительные комбинации клавиш
- Ctrl+D - переключть режим логирования (включить или выключить подробный лог)
- Ctrl+S - печать статистики работы миграции
- Ctrl+C - завершить миграцию (прерывание контекста выполнения)

### Массовое удаление объектов из JSON файла:
```bash
# Шаг 1: Получить все объекты с версиями
./minioclient -e <connection_name> -bn <bucket_name> -lbo -sv -o json > all_objects.json

# Шаг 2: Подготовить массив для удаления (все версии кроме latest)
cat all_objects.json | jq '[.[] | select(.IsLatest == false)]' > removable.json

# Шаг 3: Инициализировать удаление
./minioclient -e <connection_name> -ro -bn <bucket_name> -f -path ./removable.json
```

### Массовое удаление по тегам:
```bash
# Удалить все, кроме последней версии со всеми delete markers (для Ceph RGW)
./minioclient -e cephrgw -ro -tags '{"IsLatest":false,"IsDeleteMarker":true}' -bn archives -d -f

# Или удалить используя логику NOT (удалять если условие НЕ совпадает)
./minioclient -e cephrgw -ro -not -tags '{"IsLatest":true}' -bn archives -d -f

# Удалить объекты со специфическими тегами
./minioclient -e localdev -ro -tags '{"env":"test","cleanup":"yes"}' -bn testdata -d -f
```

### Работа через TUI:
```bash
./minioclient -i -e localdev
```

### Получение метаданных объекта:
```bash
./minioclient -e localdev -gos -bn mybucket -key myobject.txt
./minioclient -e localdev -go -bn mybucket -key myobject.txt
```

## TUI Интерфейс

Интерактивный терминальный интерфейс позволяет:
- Просматривать bucket'ы и объекты
- Управлять объектами без командной строки
- Получает информацию о бакетах
- Работать с несколькими подключениями

Пуск: `./minioclient -i -e <connection_name>`

## Примеры использования различных флагов

### Список bucket'ов в разных форматах:
```bash
# JSON вывод (по умолчанию)
minioclient -lb -o json

# Табличный вывод
minioclient -lb -o table

# Таблица с фильтрацией по префиксу и лимитом
minioclient -lb -o table -prefix docs -maxentry 50
```

### Комбинации операций:
```bash
# Получить метаданные файла в JSON формате
minioclient -e localdev -gos -bn mybucket -key myfile.txt -vid version123

# Удаление с forced режимом и dry-run
minioclient -e localdev -ro -bn mybucket -key old.txt -dryrun -f
```

## Лицензия

MIT License