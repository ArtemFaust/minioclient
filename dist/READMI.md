# Описание
Контейнер с клиентом необходим для запуска slave worker выполнения задач миграции

## Сборка контейнера для запуска воркера
```bash
docker build . -t minioclient:latest 
```
## Запуск воркера из контейнера
```bash
docker run -ti --rm --env SLAVE=true --env DEBUG=true --env MASTERENDPOINT=192.168.1.102:8080  minioclient:latest
```

## Запуск мастера из контейнера
```bash
docker run -ti --rm --env MASTER=true --env DEBUG=true --env MASTERPORT=8080 -v ./workers.db:./workers.db  minioclient:latest
```