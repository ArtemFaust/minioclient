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

**Удалить bucket:**
```bash
./minioclient -e <connection_name> -db -bn <bucket_name>
```
- `-db` – удалить существующий bucket (bucket должен быть пуст)

**Список bucket'ов:**
```bash
./minioclient -e <connection_name> -lb [-o table/json] [-prefix <prefix>] [-maxentry <count>]
```
- `-lb` – список всех bucket'ов
- `-o` – формат вывода (json/table, по умолчанию json)
- `-prefix` – фильтр по префиксу
- `-maxentry` – лимит на запись (по умолчанию 1000)

**Проверить существование bucket:**
```bash
./minioclient -e <connection_name> -bn <bucket_name>
```

**Получить информацию о bucket:**
```bash
./minioclient -e <connection_name> -bn <bucket_name>
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
| `-examples` | Показать примеры использования |
| `-e, --endpoint` | Имя подключения из конфига или FQDN (обязательный атрибут) |
| `-port` | API порт (используется из конфига если не задан явно) |
| `-accesskey` | Access key ID |
| `-secretkey` | Secret access key |
| `-ssl` | Использовать HTTPS (по умолчанию true, задать -ssl=false для HTTP) |
| `-mb` | Создать новый bucket |
| `-bn` | Название bucket |
| `-r, --region` | Регион (по умолчанию us-east-1) |
| `-db` | Удалить существующий bucket (bucket должен быть пуст) |
| `-lb` | Список всех bucket'ов |
| `-lbo` | Список объектов в конкретном bucket |
| `-liu` | Список незавершённых загрузок |
| `-po` | Загрузить новый объект (файл или директорию) |
| `-path` | Путь к файлу для загрузки |
| `-ao` | Обновить существующий объект (append) |
| `-ro` | Удалить объект по ключу, тегам или JSON файлу |
| `-key` | Ключ объекта |
| `-tags` | JSON условия удаления по тегам |
| `-not` | Инвертировать логику работы с тегами |
| `-vid` | Version ID для целевого объекта (при операции удаления/получения) |
| `-bylastmodify` | Удаление объектов кроме последнего по last-modify времени |
| `-sv` | Показывать версии файлов в списке |
| `-ls` | Unix стиль списка бакета (директории и файлы) |
| `-prefix` | Фильтр по префиксу при работе с объектами |
| `-maxentry` | Максимальное кол-во записей при списке (по умолчанию 1000) |
| `-f, --force` | Force режим операций |
| `-dryrun` | Не применять изменения (dry-run) |
| `-o, --output` | Формат вывода: json или table (по умолчанию json) |
| `-d, --debug` | Включить вывод debug сообщений |
| `-i, --interactive` | Запустить TUI интерфейс |
| `-ol` | Enable object locking для bucket |
| `-fixleak` | Исправить утечки в RGW индексе (для -ro) |
| `-leakcount` | Порог срабатывания для fixleak (по умолчанию 10) |
| `-indexpool` | Имя index pool дляRGW (по умолчанию default.rgw.buckets.index) |
| `-tpo` | Макс объектов на таблице для табличного принтера (по умолчанию 20) |
| `-dbn` | Имя бакета назначения при миграции объектов бакета. Если не указанно то имя бакета назначения будет равно имени бакета источника. |
| `-migrate` | Инициализация операции миграции объектов бакета |
| `-destination` | Название кластера куда будет выполнятся миграция объектов бакета |

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

### Массовое удаление неактуальных версий с исправлением утечек:
```bash
./minioclient -e cephrgw -ro -bn archives -bylastmodify -fixleak -leakcount 10 -d
```

### Миграция бакета из одного кластера в другой или в рамках одного кластера но в другой бакет:
```bash
./minioclient -e <source cluster from cfg> -ssl=false -migrate -destination <dst cluster from cfg> -bn <source bucket> [-dbn <dst bucket>] [-d] [-maxentry <threads count>]
```
Данная операция позволяет произвести миграцию объектов бакета в другой кластер или в тот же клстер но в другой бакет.
Если не указан destination bucket - значит будет создан бакет с названием source bucket
Каждый объект бакета обрабатывается в отдельном потоке исполнения операции миграции. Ключем maxentry можно указать сколько таких потов будет
выполнятся одновременно (сколько объектов бакета будет мигрировать одновременно). По умолчанию 1000



### Работа через TUI:
```bash
./minioclient -i -e localdev
```

## TUI Интерфейс

Интерактивный терминальный интерфейс позволяет:
- Просматривать bucket'ы и объекты
- Управлять объектами без командной строки
- Получает информацию о бакетах
- Работать с несколькими подключениями

Пуск: `./minioclient -i -e <connection_name>`

## Лицензия

MIT License