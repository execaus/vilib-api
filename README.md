# Video Library Service (Internal)

Сервис для управления видео‑материалами организации для внутреннего использования.

---

# Core (MVP)
Базовый функционал, необходимый для запуска первой версии сервиса.

## Доступ и пользователи
- [x] Авторизованный доступ к системе
- [x] Роли пользователей
  - [x] Супер администратор
  - [x] Администратор
  - [x] Модератор
  - [x] Обычный пользователь
- [x] Назначение ролей пользователям (повышение до модератора или администратора)

## Группы пользователей 
- [ ] CRUD для групп пользователей
- [ ] Назначение модераторов на группы
- [ ] Управление группами доступно модераторам
- [ ] Создание групп доступно только модератору аккаунта

## Управление видео
- [ ] CRUD операции для видео
- [x] Загрузка видео
- [x] Хранение видео в S3‑совместимом хранилище
- [ ] Привязка видео к группам пользователей
- [ ] Просмотр видео пользователями с доступом

### Диаграммы последовательности 

## Загрузка видео

Оригинал загружается клиентом напрямую в S3‑совместимое хранилище по presigned URL, без
проксирования через бэкенд. Событие о готовности оригинала бэкенд публикует сам после
подтверждения загрузки клиентом (`POST …/video/{id}/complete`) — bucket-notifications
хранилища не используются: обычный MinIO/S3 не умеет отличить финальный `PUT` большого файла
от промежуточного (multipart), а API как раз проверяет объект (`HeadObject`) перед постановкой
в очередь. Публикация в Kafka идёт через transactional outbox: запись в очередь публикации
кладётся в той же транзакции, что и перевод видео в `queued`, и публикуется relay'ем уже после
коммита — так подтверждение загрузки не зависит от доступности Kafka.

```mermaid
sequenceDiagram
  participant Client
  participant Backend
  participant S3
  participant Outbox as Outbox Relay
  participant Kafka
  participant Worker as Compression Worker Pool
  participant Postgres as DB

  %% Клиент получает presigned URL на загрузку оригинала
  Client->>Backend: POST /video (name, content_type, size_bytes)
  Backend->>S3: Presign PUT URL (ограниченное время)
  Backend->>Postgres: Создаёт video, status = uploading
  Backend-->>Client: upload_url, video_id

  %% Клиент загружает оригинал напрямую в S3 и подтверждает загрузку
  Client->>S3: PUT по upload_url
  Client->>Backend: POST /video/{id}/complete

  %% Backend проверяет объект и атомарно переводит видео в очередь + кладёт событие в outbox
  Backend->>S3: HeadObject (проверка размера/типа)
  Backend->>Postgres: Одна транзакция: asset original, video.status = queued, outbox += "video.original-uploaded"
  Backend-->>Client: 200, video (status = queued)

  %% Outbox-релей публикует событие уже после коммита транзакции
  Outbox->>Postgres: Выбирает неопубликованные события (FOR UPDATE SKIP LOCKED)
  Outbox->>Kafka: Публикует "video.original-uploaded"
  Outbox->>Postgres: Удаляет опубликованные события

  %% Worker скачивает оригинал и транскодирует по профилям
  Note over Worker: Масштабируемый пул воркеров
  Kafka-->>Worker: "video.original-uploaded"
  Worker->>S3: Скачивает оригинал
  Worker->>Kafka: "video.processing-events": ProcessingStarted
  Kafka-->>Backend: ProcessingStarted
  Backend->>Postgres: video.status = compressing (условный UPDATE: queued → compressing)

  Worker->>Worker: Транскодирует по профилям (360p/720p/…), формирует HLS-плейлисты
  Worker->>S3: Загружает hls/master.m3u8, hls/{profile}/playlist.m3u8 и сегменты

  alt Обработка успешна
    Worker->>Kafka: "video.processing-events": ProcessingCompleted (ассеты, длительность, разрешение)
    Kafka-->>Backend: ProcessingCompleted
    Backend->>Postgres: Регистрирует ассеты hls_master/hls_variant, video.status = ready
  else Ошибка обработки
    Worker->>Kafka: "video.processing-events": ProcessingFailed (класс, причина)
    Kafka-->>Backend: ProcessingFailed
    Backend->>Postgres: video.status = failed (класс permanent/timeout), либо — для временной ошибки с запасом попыток — снова queued и повторная публикация "video.original-uploaded"
  end

  Note over Backend,Postgres: Watchdog независимо от Kafka переводит в failed(timeout) видео, зависшие<br/>в uploading/queued/compressing дольше сконфигурированных таймаутов
```

## Получение видео

Готовое видео отдаётся через HLS: бэкенд не хранит и не проксирует сегменты, а на лету
переписывает плейлисты, читая их из хранилища и подписывая ссылки на сегменты presigned
URL (бакет остаётся закрытым). Доступ к плейлистам проверяется отдельным короткоживущим
HLS-токеном в query, а не заголовком `Authorization` — плееры (hls.js, нативный Safari) сами
запрашивают master → variant playlists → сегменты и не могут добавлять произвольные заголовки
к каждому запросу.

```mermaid
sequenceDiagram
    participant Client
    participant Backend
    participant Postgres as DB
    participant S3

    Client->>Backend: GET /video/{id}?is_prefer_original=false
    Backend->>Postgres: Читает статус видео и его ассеты
    Postgres-->>Backend: status, ассеты (original / hls_master / hls_variant)

    alt status = ready, есть hls_master и prefer_original = false
        Backend-->>Client: kind = "hls", URL master.m3u8 с HLS-токеном (JWT, ограниченный TTL)
    else оригинал загружен (uploading/queued/compressing/failed с оригиналом, либо prefer_original = true)
        Backend->>S3: Presign GET URL на оригинал (ограниченное время)
        S3-->>Backend: presigned URL
        Backend-->>Client: kind = "original", presigned URL
    else оригинала нет
        Backend-->>Client: 409 video is not available
    end

    opt kind = "hls"
        Client->>Backend: GET /video/{id}/hls/master.m3u8?token=…
        Backend->>S3: GetObject(master.m3u8)
        Backend-->>Client: master-плейлист, ссылки на варианты переписаны с тем же токеном

        Client->>Backend: GET /video/{id}/hls/{profile}/playlist.m3u8?token=…
        Backend->>S3: GetObject(playlist профиля)
        Backend->>S3: Presign GET URL на каждый сегмент плейлиста
        Backend-->>Client: медиаплейлист с преподписанными URL сегментов

        Client->>S3: GET по преподписанным URL сегментов
        S3-->>Client: Отдаёт сегменты напрямую
    end

    opt kind = "original"
        Client->>S3: GET presigned URL
        S3-->>Client: Отдаёт оригинал видео
    end
```

### Заметки

* добавить тесты на работы с uint64 в postgres
* добавить везде проверку на выбранный account id в claims и фактический в url

### Возможности по статусам

#### Статусы аккаунтов

| Статус      | Возможности                                                                                                                                  |
|-------------|----------------------------------------------------------------------------------------------------------------------------------------------|
| Super Admin | Создаёт аккаунты, назначает/снимает любые роли, включая других админов; при передаче супер админа снижается до админа; все права наследуются |
| Admin       | Добавляет пользователей в аккаунт (статус по умолчанию user), назначает/снимает модераторов;                                                 |
| Moderator   | Создаёт и удаляет группы; добавляет пользователей в группы                                                                                   |
| User        | Только просмотр видео и доступных групп                                                                                                      |

#### Статусы внутри групп

| Статус группы | Возможности                                                                                      |
|---------------|--------------------------------------------------------------------------------------------------|
| Moderator     | Добавляет, изменяет и удаляет видео в группе; управляет составом группы (назначение модераторов) |
| User          | Просмотр видео в группе, добавление и удаление видео (только своих)                              |

> ⚠️ Все права наследуются сверху вниз: super admin > admin > moderator > user, но права в группах определяются отдельно и не автоматически наследуются от статуса аккаунта.
---

# Moderation / Extensions
Дополнительный функционал, который можно реализовать после MVP.

## Обработка видео
- [x] Сжатие видео

## Модерация контента
- [ ] Заявка на добавление видео
- [ ] Проверка заявки модератором
- [ ] Принятие решения о публикации видео

## Уведомления
- [ ] Оповещения пользователей о добавлении новых видео

## AI функции
- [ ] Автоматическое создание тайм‑кодов для видео

---

## Генерация кода (bob)

Для генерации кода через **bob** необходимо создать конфигурационный файл `bobgen.yaml` в корне проекта.

Пример конфигурации (без конфиденциальных данных):

```yaml
psql:
  dsn: "postgres://<user>:<password>@<host>:<port>/<database>?sslmode=disable"
  driver: "github.com/jackc/pgx/v5/stdlib"
  schemas:
    - "app"
  uuid_pkg: "google"
  queries:
    - ./internal/repository

plugins:
  dbinfo:
    disabled: true
  enums:
    disabled: true
  models:
    disabled: false
    pkgname: "schema"
    destination: "./internal/gen/schema"
  factory:
    disabled: true
  dberrors:
    disabled: false
    pkgname: "dberrors"
    destination: "./internal/gen/dberrors"
  where:
    disabled: true
  loaders:
    disabled: true
  joins:
    disabled: true
```
