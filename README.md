# Distributed computing and applications

Лабораторные выполняются последовательно в ветках `lab/XX-<name>`, коммиты — `lab-XX: <description>`.
Реальные настройки хранятся в игнорируемых `<service>/.env`; в Git добавляются только `example.env`.

## Запуск

Скопируйте `example.env` в `.env` в каталогах `database`, `employees`, `reports`, `files`, `storage`, `queue`, `kafka-ui`, `monitoring`.
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
docker compose exec -T files-1 wget -qO- http://localhost:8080/api/v1/files/FILE_ID
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

### Kafka UI и Grafana

| Страница | Что показать |
|---|---|
| [Swagger](http://localhost:8080/employees/swagger/index.html) | JWT, CRUD сотрудников, отчёт и скачивание CSV |
| [Kafka UI](http://localhost:8081) | Кластер `local`, топик `reports.files`, сообщения, группа `files-service` и lag |
| [Grafana — Services overview](http://localhost:3000/d/services/services-overview) | Состояние и метрики трёх сервисов |
| [Prometheus Targets](http://localhost:9090/targets) | Проверка, что все сервисы имеют статус UP |
| [Jaeger](http://localhost:16686/search) | Трасса HTTP → Kafka → S3 |
| [Kibana — Discover](http://localhost:5601/app/discover) | Поиск логов всех сервисов по `service`, `level`, `trace_id`, `file_id` |

Kafka UI — [Kafbat UI](https://ui.docs.kafbat.io/overview/getting-started), заранее подключён к `kafka:9092`.
Вход в Grafana — логин и пароль `GF_SECURITY_ADMIN_USER` / `GF_SECURITY_ADMIN_PASSWORD`
из локального `monitoring/.env`. Не используйте пароль приложения вместо пароля Grafana.
При первом запуске Grafana создаёт учётную запись; изменение `.env` после этого само по себе пароль не меняет.

Prometheus и дашборд подключаются через [file provisioning Grafana](https://grafana.com/docs/grafana/latest/administration/provisioning/).
Ручной импорт не нужен. В дашборде шесть панелей: доступность сервисов, число запросов за выбранный
интервал, запросы в секунду, p95 времени ответа, ошибки HTTP 4xx/5xx и память RSS.
Сверху можно выбрать сервис, период и перейти в остальные инструменты демонстрации.
Обновление — каждые 5 секунд. p95 и частота запросов отражают интервалы с реальным трафиком.
Создайте несколько отчётов и скачайте CSV через Swagger, чтобы показать изменения на графиках.

Дашборд хранится в `monitoring/grafana/dashboards/services.json`, настройки источника и загрузки —
в `monitoring/grafana/provisioning`. Правки дашборда делаются в JSON. Данные Grafana сохраняются
в volume `grafana_data`; пароли — только в игнорируемом `.env`. Kafka UI и Grafana доступны на localhost.

### Логирование

```sh
docker compose logs -f employees-1 employees-2 reports-1 reports-2 files-1 files-2
python3 scripts/smoke.py
```

Все три Go-сервиса (`employees-service`, `reports-service`, `files-service`) пишут в stdout
по одной JSON-строке на событие. Общий `common/telemetry.InitLogging` вызывается до загрузки
конфигурации: запуск и ошибки настроек тоже имеют поля `time`, `level`, `msg`, `service`.
Логи из контекста HTTP/Kafka автоматически получают `trace_id` и `span_id`.
HTTP middleware добавляет `method`, шаблон `route`, `status`, `duration_ms`: 2xx/3xx — INFO,
4xx — WARN, 5xx — ERROR. `/metrics` не засоряет логи. Паника обработчика записывается
как ERROR с `error` и `stack`, затем клиент получает 500.

Ошибки БД логируются на границе HTTP с контекстом трассы. Стандартный вывод GORM отключён,
чтобы SQL с ФИО и другими значениями не попадал в лог. Прикладные ошибки, ошибки записи ответа
и HTTP-сервера также используют JSON. Продюсер Kafka пишет `published queued report`,
потребитель — `saved queued report`; обе записи содержат `file_id` и общий `trace_id`.
Повреждённое сообщение — WARN с топиком, партицией и offset, сбой сохранения — ERROR.
Пароли, JWT, заголовки авторизации и тела запросов не записываются.

Цепочка сбора: **stdout/stderr → Docker json-file → Filebeat → Logstash → Elasticsearch → Kibana**.
[Filebeat autodiscover](https://www.elastic.co/docs/reference/beats/filebeat/configuration-autodiscover)
выбирает контейнеры с меткой `distributed.logs=json`; в Compose она включена у трёх Go-сервисов.
Logstash разбирает JSON, преобразует `time` в `@timestamp`, `msg` в `message` и индексирует
события в `app-logs-YYYY.MM.dd`. Неожиданный текстовый вывод сохраняется с `_jsonparsefailure`
и уровнем ERROR. Типы полей заданы шаблоном: `trace_id`, `service`, `level` и `file_id` доступны
для точного поиска и группировок, `message` и `error` — для полнотекстового поиска.

В [Kibana Discover](http://localhost:5601/app/discover) представление **Application logs**
(`app-logs-*`) создаётся автоматически через `logging-setup` и выбирается по умолчанию.
После запроса отчёта откройте Discover, выберите время **Last 15 minutes**, нажмите Refresh.
Примеры запросов KQL в строке поиска:

```text
service: "employees-service"
level: "ERROR"
status >= 400
trace_id: "<trace_id из лога или smoke.py>"
file_id: "<file_id отчёта>"
service: "files-service" and message: "saved queued report"
```

Колонки `service`, `level`, `message`, `status`, `trace_id` настраиваются автоматически.
По одному `trace_id` видны HTTP-вызовы, публикация в Kafka и сохранение CSV;
этот же ID можно открыть в Jaeger: `http://localhost:16686/trace/<trace_id>`.
Конфигурация стека лежит в `logging/`, готовые настройки индекса и представления — в `logging/setup/`.
Elasticsearch API для проверки — [localhost:9200](http://localhost:9200).

Индексы сохраняются в volume `elasticsearch_data`, позиции чтения и дисковые очереди —
в `filebeat_data` и `logstash_data`. У Filebeat и Logstash очереди по 256 MiB с повторной отправкой
при недоступности Elasticsearch. Доставка допускает повторы после аварийного завершения.
Docker ротирует исходные логи (3 файла по 10 MiB на сервис); длительный простой сборщика
может привести к утрате ещё не прочитанных записей после ротации. Индексы автоматически
не удаляются: для длительной эксплуатации нужна политика хранения и контроль свободного места.
`docker compose down` сохраняет историю; **`docker compose down -v` удаляет volumes вместе с данными**.

ELK запускается вместе со стендом через `docker compose up -d --build`.
Первый запуск требует загрузки крупных образов и может занять несколько минут;
Docker Desktop рекомендуется выделить не менее 8 GB памяти для всего стенда.
Это локальный одновузловой ELK без TLS и отдельной авторизации, порты 5601 и 9200
привязаны к localhost. Для удалённого доступа нужно включить безопасность Elastic.
Filebeat использует доступ к Docker API и каталогу логов; стенд рассчитан на доверенную локальную машину.

`smoke.py` читает учётные данные из локальных `employees/.env` и `monitoring/.env`, проверяет JWT,
CSV, цели Prometheus, трассу HTTP → Kafka → S3, чтение топика через Kafka UI и запросы всех
шести панелей Grafana к Prometheus, индексирование JSON-логов трёх сервисов с общим `trace_id`
и настройку Kibana; секреты не печатает.
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

## Несколько инстансов: Docker DNS и Nginx

По умолчанию запущено по два инстанса каждого Go-сервиса. Внутри сети адреса
`http://employees-service`, `http://reports-service`, `http://files-service` ведут на Nginx
(порт 80), который распределяет HTTP-запросы round-robin. Приложения слушают 8080.
Внешний адрес Swagger остаётся прежним; публичный listener Nginx отделён от внутреннего.
Метрики доступны Prometheus напрямую и не открываются через gateway.

Общие настройки инстансов объявлены в шаблонах Compose. Чтобы добавить ещё один,
добавьте в `services` строку `employees-3: *employees-instance` и выполните
`docker compose up -d --build employees-3`. Для других типов используются
`*reports-instance` и `*files-instance`. Шаблон включает общий backend-алиас;
Nginx обновляет список адресов через Docker DNS каждые 5 секунд без reload.
Также можно масштабировать один блок: `docker compose up -d --scale employees-1=3`.
После добавления нового типа приложения ему потребуется отдельный upstream и логический адрес.

Из `employees/example.env` и `reports/example.env` перенесите порты HTTP-клиентов `80`
в локальные `.env`; секреты сохраняются в прежних игнорируемых файлах.
Первый переход со старых имён контейнеров: `docker compose up -d --build --remove-orphans`.
Общая БД, S3 и одинаковые JWT-настройки позволяют обслуживать запрос на любом инстансе.
Заголовок `X-Service-Instance`, поле JSON `instance_id` и атрибут трассы
`service.instance.id` показывают конкретный контейнер. Prometheus использует DNS discovery
для каждого backend, Grafana различает экземпляры в панелях доступности и памяти.

`/healthz` проверяет запуск HTTP-сервера и не создаёт логов/трасс. Docker DNS не фильтрует
`unhealthy`; Nginx использует пассивное исключение сбоев на 5 секунд. Во время обновления DNS
или отказа возможен ответ 502/504. Автоматические повторы отключены, поскольку даже GET отчёта
публикует сообщение Kafka. После удаления инстанса дождитесь обновления DNS.
Nginx, PostgreSQL и управляющий узел S3 пока остаются отдельными точками отказа.

Проверка балансировки, добавления и остановки инстанса: `python3 scripts/check_instances.py`.
Скрипт удаляет созданный тестовый контейнер и восстанавливает остановленную реплику.
