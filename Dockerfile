# syntax=docker/dockerfile:1.7

# Стадия сборки: go.mod зависит от приватного github.com/execaus/vilib-events — доступ к нему
# в контейнере даёт SSH-форвардинг BuildKit (`docker build --ssh default ...`), не токены и не
# ключи в образе.
FROM golang:1.25-alpine AS build

RUN apk add --no-cache git openssh-client ca-certificates

ENV GOPRIVATE=github.com/execaus/* \
    CGO_ENABLED=0

RUN git config --global url."ssh://git@github.com/".insteadOf "https://github.com/" \
    && mkdir -p -m 0700 ~/.ssh \
    && ssh-keyscan github.com >> ~/.ssh/known_hosts

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=ssh \
    --mount=type=cache,target=/root/go/pkg/mod \
    go mod download

COPY . .

RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/root/go/pkg/mod \
    go build -trimpath -ldflags="-s -w" -o /out/vilib-api ./cmd

# goose собирается отдельной командой из своего модуля (а не как сторонний build-tool этого
# репозитория): go.mod vilib-api тянет только библиотеку github.com/pressly/goose/v3, а
# `cmd/goose` дополнительно требует драйверы всех поддерживаемых СУБД (clickhouse, ydb, mssql
# и т.д.) — раздувать go.sum этого модуля ради утилиты миграций нецелесообразно.
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/root/go/pkg/mod \
    GOBIN=/out go install github.com/pressly/goose/v3/cmd/goose@v3.27.0

# Стадия migrate: бинарь goose + папка миграций — контракт для vilib-deploy
# (build: {context: ../vilib-api, target: migrate}, §10.1 дизайна эпика). Применяет миграции
# и завершается с кодом 0 (или ненулевым при ошибке) — постоянного процесса не запускает.
FROM alpine:3.21 AS migrate

RUN apk add --no-cache ca-certificates

COPY --from=build /out/goose /usr/local/bin/goose
COPY migrations /migrations
COPY docker/migrate-entrypoint.sh /usr/local/bin/migrate-entrypoint.sh

ENTRYPOINT ["/usr/local/bin/migrate-entrypoint.sh"]

# Финальная стадия: сам API. Конфиг-файл в образ не копируется — все настройки приходят через
# переменные окружения (config.LoadConfig честно работает без файла).
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -u 10001 app

COPY --from=build /out/vilib-api /usr/local/bin/vilib-api

USER app

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/vilib-api"]
