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
Опубликованные порты привязаны к localhost.
Сервисы отчётов, файлов и S3 доступны только внутри Docker-сети.
В ветке №7 для API сначала получите JWT по инструкции ниже и добавляйте `Authorization: Bearer <token>`.
Для сборки Docker используется корень репозитория: `docker build -f employees/Dockerfile .`.

## Лабораторная 4 — файловое хранилище

```sh
curl http://localhost:8080/employees/api/v1/reports/employees-report
```

Основной сервис запрашивает отчёт по HTTP. Сервис отчётов получает сотрудников из основного
сервиса, формирует CSV (`id,fio,birth_date,is_fired`) и в ветке №4 передаёт его по HTTP файловому сервису.
Тот сохраняет CSV в S3-совместимый SeaweedFS. Ответ содержит `file_id`, `employees`, `total`, `is_fired`.
Лимит одного CSV — 8 MiB. Для пустого списка сохраняется CSV с заголовком.

В ветке №4 скачивание доступно внутри сети (в №7 этот запрос также требует JWT):

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

## Лабораторная 6 — Future на Go

В проекте на Go роль `CompletableFuture` выполняют goroutine и буферизированный канал результата.
`GetAsync` сразу возвращает канал; основной сервис ожидает CSV через `select`, учитывая отмену запроса
и таймаут 5 секунд. Запрос к файловому сервису использует тот же контекст. Буфер канала позволяет
goroutine завершиться, даже если клиент уже отключился.

Добавьте `FILES_CLIENT_HOST`/`FILES_CLIENT_PORT` из `employees/example.env` в локальный `.env`.
После получения `file_id` скачайте CSV через шлюз:

```sh
curl -OJ http://localhost:8080/employees/api/v1/reports/files/FILE_ID
```

Неизвестный или ещё не обработанный файл возвращает 404, неверный UUID — 400,
сбой файлового сервиса — 502, истечение времени ожидания — 504.

## Лабораторная 7 — JWT и наблюдаемость

В `employees/.env` задайте `AUTH_USERNAME`, `AUTH_PASSWORD`. Во всех трёх сервисах укажите одинаковый
`JWT_SECRET` длиной не менее 32 байт. Значения не включаются в Git и Docker build context.
Пароль локальной учётной записи хранится в `employees/.env`.

```sh
curl http://localhost:8080/employees/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"<AUTH_USERNAME>","password":"<AUTH_PASSWORD>"}'
```

Этот запрос доступен и в Swagger: **auth → POST /auth/login → Try it out**.
Прежний адрес `/employees/auth/login` также работает.
Сохраните `access_token` из ответа в переменную `TOKEN`. Токен действует 15 минут.

```sh
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/employees/api/v1/reports/employees-report
curl -OJ -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/employees/api/v1/reports/files/FILE_ID
```

В Swagger нажмите **Authorize** и вставьте `Bearer <access_token>`.
Все бизнес-эндпоинты трёх сервисов проверяют подпись HS256, срок действия, issuer, audience и subject.
Токен передаётся только по внутренним HTTP-запросам; в сообщения Kafka он не включается.
Учётная запись одна, настраивается через env; регистрации, ролей и refresh-токенов нет.

`OTEL_EXPORTER_OTLP_ENDPOINT=http://jaeger:4318` включает экспорт трасс.
OpenTelemetry передаёт W3C `traceparent` через HTTP, Kafka и обращения к S3.
В [Jaeger](http://localhost:16686) выберите `employees-service` и запрос отчёта:
в одной трассе будут основной сервис, отчёты, публикация/обработка очереди и запись S3.
При скачивании создаётся отдельная трасса основного и файлового сервисов.
Jaeger хранит трассы в памяти, поэтому перезапуск очищает их.

[Prometheus](http://localhost:9090/targets) собирает `/metrics` всех трёх сервисов каждые 5 секунд.
Доступны `http_requests_total`, `http_request_duration_seconds` и метрики Go/process.
Метки используют шаблоны маршрутов, поэтому UUID не создают новые временные ряды.
Метрики не публикуются через шлюз; UI Prometheus и Jaeger доступны только на localhost.

```sh
docker compose logs -f employees-service reports-service files-service
python3 scripts/smoke.py
```

Логи запросов — JSON с сервисом, маршрутом, статусом, длительностью и `trace_id`.
Лог потребителя содержит `file_id` и тот же `trace_id`. Пароли, JWT и тела запросов не логируются.
`smoke.py` читает учётные данные из локального `employees/.env`, проверяет JWT, CSV, все три
цели Prometheus и трассу HTTP → Kafka → S3 в Jaeger; секреты не печатает.
Стенд использует HTTP и предназначен для локальной демонстрации; внешний доступ требует TLS.

Проверки модулей:

```sh
for service in common employees reports files; do
  (cd "$service" && go test -race ./...)
done
```

Основной Compose сохраняет порт PostgreSQL 5432 для локальной разработки.
Если он занят другим приложением, освободите его или используйте временный Compose override,
не меняя адрес БД внутри Docker-сети (`postgres:5432`).
