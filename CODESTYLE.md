# Code Style Guide — Vilib API

---

## 1. Структура файлов

- Каждый handler — в отдельном файле, имя файла соответствует действию: `create_user.go`, `get_video.go`, `upload_video.go`
- Каждый сервис — в отдельном файле: `video.go`, `account.go`
- Ошибки сервиса — в отдельном файле рядом: `video_errors.go`, `account_errors.go`
- Тест — рядом с тестируемым файлом: `video.go` → `video_test.go`
- Mock-файлы генерируются в папках `service_mocks/` и `repository_mocks/` и не редактируются вручную

---

## 2. Именование

- Файлы — `snake_case`
- Типы, функции и методы именуются согласно спецификации Go: публичные — `PascalCase`, приватные — `camelCase`
- Sentinel-ошибки — `ErrXxx`: `ErrNotFound`, `ErrForbidden`
- Типы ошибок — `XxxError`: `ConflictError`, `ForbiddenError`
- Конструкторы — `NewXxx(...)`: `NewVideoService(...)`, `NewHandler(...)`
- Переменные URL — `xxxURL`: `UploadVideoURL`, `GetVideoURL`
- Константы PathKey — `pathKeyXxx`: `pathKeyAccountID`, `pathKeyVideoID`

---

## 3. Слой Handler

Единый паттерн для каждого handler'а:

1. Извлечь path-параметры через `h.GetPathUUIDValue()`
2. Забиндить тело запроса через `c.BindJSON()` или query через `c.ShouldBindQuery()`
3. Выполнить бизнес-логику через `h.saga.Run()`
4. Вернуть ответ через `sendOK()`, `sendCreated()` или ошибку через `sendServiceError()` / `sendBadRequest()`

В handler'е не должно быть бизнес-логики — только транспортный код.

Swagger godoc обязателен для каждого handler'а, комментарии на русском языке.

---

## 4. Слой Service

- Конструктор принимает зависимости явно: репозиторий + `*Service` для межсервисных вызовов
- Первым шагом в методе — проверка прав доступа, затем бизнес-логика
- Все ошибки логируются через `zap.L().Error(err.Error())` перед возвратом
- Бизнес-ошибки определяются в отдельном файле `xxx_errors.go` рядом с сервисом
- Интерфейс каждого сервиса объявляется в `service.go`, там же размещаются директивы `//go:generate minimock`

---

## 5. Слой Repository

- Методы называются по выполняемой операции с БД: `Select`, `Insert`, `Update`, `Delete` — не `Find`, `Create`, `Save`
- `pgx.ErrNoRows` оборачивается в `repository.ErrNotFound` на уровне репозитория
- Конвертация из генерированной схемы в domain-сущность — через метод `.FromDB(db *schema.Xxx)` на структуре domain
- Ошибки не логируются в репозитории — логирование происходит на уровне сервиса

---

## 6. Обработка ошибок

- `ErrNotFound` — объявляется в `service/service_errors.go`, используется как sentinel через `errors.Is`
- `ErrForbidden` — тип `*ForbiddenError`, создаётся через `NewForbiddenError("...")`, handler преобразует в HTTP 403
- `ConflictError` — создаётся через `NewConflictError("...")`, handler преобразует в HTTP 409
- Handler конвертирует ошибки сервиса в HTTP-ответы через `sendServiceError()`
- При HTTP 500 тело ответа не содержит деталей ошибки — только статус

---

## 7. Тестирование

Все тесты пишутся в стиле **table-driven**.

Структура теста:

```go
tests := []struct {
    name       string
    setupMocks func(...)
    args       args
    want       SomeType
    wantErr    error
}{
    {
        name: "success",
        setupMocks: func(...) { ... },
        args:    args{ ... },
        want:    expectedValue,
    },
    {
        name:    "some error case",
        setupMocks: func(...) { ... },
        args:    args{ ... },
        wantErr: ErrSomething,
    },
}

for _, tt := range tests {
    tt := tt
    t.Run(tt.name, func(t *testing.T) {
        t.Parallel()
        // ...
    })
}
```

### Handler-тесты

- Пакет: `handler_test` (внешний тестовый пакет)
- Роутер создаётся через `testutil.SetupTestRouterWithMocks()`
- HTTP-запрос собирается через `httptest.NewRequest`, ответ — через `httptest.NewRecorder`
- Проверка статуса: `require.Equal(t, http.StatusXxx, w.Code)`
- Контроллер моков: `mc := minimock.NewController(t)` — завершается автоматически через `t.Cleanup`

### Service-тесты

- Пакет: `service_test` (внешний тестовый пакет)
- Каждый подтест запускается параллельно: `t.Parallel()`
- Контекст в ожиданиях мока — `minimock.AnyContext`
- Проверка ошибок: `require.ErrorIs(t, err, tt.wantErr)`
- Контроллер моков: `mc := minimock.NewController(t)` — завершается автоматически через `t.Cleanup`

---

## 8. Комментарии

- Публичные типы, функции и методы — комментарий обязателен
- Swagger godoc на handler'ах — на русском языке
- Комментарии внутри методов сервиса — на русском, кратко описывают шаг бизнес-логики
- Комментарии к константам и полям domain-типов — на русском

---

## 9. Форматирование и линтер

- Форматтер — `goimports` (import-префикс `github.com/execaus/vilib`)
- Максимальная длина строки — **120 символов** (`golines`)
- Линтер — `golangci-lint`, конфигурация в `.golangci.yml` в корне репозитория
