# minioclient

CLI клиент для управления объектным хранилищем MinIO с поддержкой TUI интерфейса.

## Особенности

- Управление bucket'ами (создание, удаление, список)
- Работа с объектами (загрузка, удаление, получение метаданных)
- Встроенная TUI консоль
- Поддержка нескольких endpoint'ов через конфигурацию
- JSON и табличный вывод результатов
- Dry-run режим для безопасного тестирования
- Работает с S3 API

## Требования

- Go 1.21+
- MinIO сервер или совместимое S3 хранилище

## Установка

```bash
go build -o minioclient .
```

## Использование

### Быстрый старт

#### 1. Создайте конфигурационный файл `config.yaml`:

```yaml
name: local_minio
port: "9000"
accesskey: "minioadmin"
secretkey: "minioadmin"
ssl: false
endpoints:
  - http://localhost:9000
  - http://127.0.0.1:9000
```

#### 2. Основные команды:

**Создать bucket:**
```bash
./minioclient -e local_minio -mb -bn mybucket
```

**Список bucket'ов:**
```bash
./minioclient -e local_minio -lb
```

**Загрузить объект:**
```bash
./minioclient -e local_minio -po -bn mybucket -path /path/to/file
```

**Удалить объект:**
```bash
./minioclient -e local_minio -ro -bn mybucket -key file.txt
```

**Список объектов:**
```bash
./minioclient -e local_minio -lbo -bn mybucket
```

### TUI Режим

Запустите интерактивный интерфейс:

```bash
./minioclient -i -e local_minio
```

## Настройка

Файл конфигурации `config.yaml` определяет подключения к MinIO. Поддерживается несколько endpoint'ов для повышения доступности.

## Аргументы командной строки

| Флаг | Описание |
|------|----------|
| `-e` | Имя подключения из конфига или FQDN |
| `-mb` | Создать bucket |
| `-bn` | Название bucket |
| `-lb` | Список bucket'ов |
| `-po` | Загрузить объект |
| `-path` | Путь к файлу для загрузки |
| `-ro` | Удалить объект |
| `-key` | Ключ объекта |
| `-lbo` | Список объектов в bucket |
| `-i` | Запустить TUI |
| `-d` | Включить debug вывод |
| `-dryrun` | Не применять изменения |

## Примеры конфигурации

```yaml
name: production_minio
port: "9000"
accesskey: "your-access-key"
secretkey: "your-secret-key"
ssl: true
endpoints:
  - https://api.minio.example.com
  - https://backup.minio.example.com
```

## Лицензия

MIT License
