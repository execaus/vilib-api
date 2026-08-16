#!/bin/sh
# migrate-entrypoint.sh — точка входа образа стадии `migrate` (контракт для vilib-deploy,
# §10.1 дизайна эпика: build.target: migrate).
#
# Применяет миграции goose к БД, заданной через GOOSE_DRIVER/GOOSE_DBSTRING (goose поддерживает
# оба параметра как переменные окружения — позиционные аргументы драйвера и DSN не нужны).
#
# Таблицы приложения в миграциях объявлены с явной квалификацией схемы (app.<table>), поэтому
# search_path на их размещение не влияет. Исключение — служебная таблица версий
# goose_db_version: сама библиотека goose создаёт её без схемы, и она попадает в ту схему,
# что первая в search_path подключения. Чтобы поведение совпадало 1:1 с testutil.WithDB (там
# миграции применяются отдельным подключением с search_path=public, а не app), при отсутствии
# параметра search_path в GOOSE_DBSTRING скрипт добавляет "search_path=public" явно.
set -eu

if [ -z "${GOOSE_DBSTRING:-}" ]; then
    echo "GOOSE_DBSTRING is not set" >&2
    exit 1
fi

case "$GOOSE_DBSTRING" in
    *search_path=*)
        ;;
    *\?*)
        GOOSE_DBSTRING="${GOOSE_DBSTRING}&search_path=public"
        ;;
    *)
        GOOSE_DBSTRING="${GOOSE_DBSTRING}?search_path=public"
        ;;
esac
export GOOSE_DBSTRING

exec goose -dir /migrations up
