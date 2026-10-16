# Distributed computing and applications

Лабораторные выполняются последовательно в ветках `lab/XX-<name>`, коммиты — `lab-XX: <description>`.
Реальные настройки хранятся в игнорируемых `<service>/.env`; в Git добавляются только `example.env`.

## Запуск

Скопируйте `example.env` в `.env` в каталогах `database`, `employees`, `reports`, `files`, `storage`, `queue`.
В `files/.env` и `storage/.env` должны совпадать S3-ключи и имя бакета. Замените демонстрационные пароли.

```sh
docker compose up -d --build
```

Шлюз: `http://localhost:8080`. Swagger: `http://localhost:8080/employees/swagger/index.html`.
Сервисы отчётов, файлов и S3 доступны только внутри Docker-сети.

## Лабораторная 4 — файловое хранилище

```sh
curl http://localhost:8080/employees/api/v1/reports/employees-report
```

Основной сервис запрашивает отчёт по HTTP. Сервис отчётов получает сотрудников из основного
сервиса, формирует CSV (`id,fio,birth_date,is_fired`) и в ветке №4 передаёт его по HTTP файловому сервису.
Тот сохраняет CSV в S3-совместимый SeaweedFS. Ответ содержит `file_id`, `employees`, `total`, `is_fired`.
Лимит одного CSV — 8 MiB. Для пустого списка сохраняется CSV с заголовком.

Пока скачивание доступно внутри сети:

```sh
docker compose exec -T files-service wget -qO- http://localhost:8080/api/v1/files/FILE_ID
```

Проверка: `python3 scripts/smoke.py`. В каждом Go-модуле: `go test ./...`.

## Лабораторная 5 — очередь

В `lab/05-queue` HTTP-передача CSV заменена Kafka (один узел в режиме KRaft).
`QUEUE_BROKERS` и `QUEUE_TOPIC` в `reports/.env` и `files/.env` должны совпадать;
топик из `queue/.env` автоматически создаётся `kafka-init` при запуске Compose.
Продюсер ожидает подтверждения Kafka (`acks=all`). `file_id` возвращается после записи сообщения,
поэтому до обработки скачивание может вернуть 404. Потребитель фиксирует offset только после записи в S3.
При сбое контейнер перезапускается и повторно читает неподтверждённое сообщение с тем же UUID;
запись по тому же ключу позволяет безопасно повторить доставку.
Повреждённые сообщения пропускаются с записью в лог, чтобы не блокировать партицию.
Лимит CSV — 8 MiB; лимиты Kafka учитывают накладные расходы сообщения.
Данные Kafka сохраняются в Docker volume. Один брокер не обеспечивает отказоустойчивость при потере диска.
