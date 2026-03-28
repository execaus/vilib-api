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
- [ ] Загрузка видео
- [ ] Хранение видео в S3‑совместимом хранилище
- [ ] Привязка видео к группам пользователей
- [ ] Просмотр видео пользователями с доступом

### Диаграммы последовательности 

## Загрузка видео

```mermaid
sequenceDiagram
  participant Client
  participant Backend
  participant S3
  participant Kafka
  participant Worker as Compression Worker Pool
  participant Postgres as DB

%% Клиент получает presigned URL
  Client->>Backend: Запрос на загрузку видео
  Backend->>S3: Запрос presigned URL (действует ограниченное время)
  Backend->>Postgres: Создает video со статусом uploading
  S3-->>Backend: Возвращает presigned URL
  Backend-->>Client: Отдаёт presigned URL

%% Клиент загружает видео, S3 отправляет событие в Kafka
  Client->>S3: Загружает видео по URL
  S3-->>Kafka: Событие "UploadCompleted" (оригинал)

%% Backend получает событие завершения первой загрузки
  Kafka-->>Backend: Видео загружено
  Backend->>Postgres: Создаёт asset видео с тегом original
  Backend->>Kafka: Событие "OriginalUploaded"

%% Worker скачивает и сжимает видео
  Note over Worker: Масштабируемый пул воркеров
  Kafka-->>Worker: Событие "OriginalUploaded"
  Worker->>Worker: Скачивает и начинает сжатие
  Worker->>Kafka: Событие "CompressionStarted"
  Kafka-->>Backend: Событие "CompressionStarted"
  Backend->>Postgres: Обновляет запись video.status = compressing
  Worker->>Worker: Сжимает видео
  Worker->>S3: Загружает сжатое видео
  Worker->>Kafka: Событие "CompressionCompleted"
  Kafka-->>Backend: Событие "CompressionCompleted"
  Backend->>Postgres: Создаёт asset для сжатой версии, обновляет status = ready
```

## Получение видео

```mermaid
sequenceDiagram
    participant Client
    participant Backend
    participant Postgres as DB
    participant S3

    %% Клиент запрашивает ссылку на видео
    Client->>Backend: GET /video?video_id=123&prefer_original=false
    Backend->>Postgres: Проверить статус видео (есть ли сжатое)
    Postgres-->>Backend: Видео: original_uploaded, compressed_ready

    %% Backend решает, какой URL отдавать
    alt Сжатое видео готово и prefer_original=false
        Backend->>S3: Сгенерировать presigned URL для сжатого видео (время жизни ограничено)
        S3-->>Backend: Возвращает presigned URL
        Backend-->>Client: Отдаёт URL на сжатое видео
    else Сжатое видео не готово или prefer_original=true
        Backend->>S3: Сгенерировать presigned URL для оригинала (время жизни ограничено)
        S3-->>Backend: Возвращает presigned URL
        Backend-->>Client: Отдаёт URL на оригинал
    end

    %% Клиент напрямую загружает видео с S3
    Client->>S3: GET presigned URL
    S3-->>Client: Отдаёт видео
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
- [ ] Сжатие видео

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
