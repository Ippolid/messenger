# Строка подключения к локальной БД из docker-compose
DB_DSN ?= postgres://messenger:messenger@localhost:5432/messenger?sslmode=disable
COMPOSE := docker compose -f deploy/docker-compose.yml

# CLI-инструменты запускаются через `go tool` — их НЕ нужно ставить в PATH,
# они объявлены как tool-зависимости в go.mod и качаются автоматически.
BUF          := go tool buf
# migrate — собственная обёртка cmd/migrate (собирается с postgres+file драйверами)
MIGRATE      := go run ./cmd/migrate
GOLANGCILINT := go tool golangci-lint

.PHONY: dev up down build proto migrate-up migrate-down seed run web lint test

## dev: поднять всё окружение одной командой (up + proto + миграции)
dev: up proto migrate-up
	@echo " окружение готово: PostgreSQL и Redis подняты, proto сгенерирован, схема применена"
	@echo "   дальше: make run, затем открой http://localhost:8080"

## up: поднять PostgreSQL и Redis и дождаться готовности (--wait ждёт healthcheck)
up:
	$(COMPOSE) up -d --wait

## down: остановить окружение (данные в volume сохраняются)
down:
	$(COMPOSE) down

## build: собрать все бинарники
build:
	go build ./...

## proto: сгенерировать gRPC-код из proto
proto:
	$(BUF) generate

## migrate-up: применить миграции
migrate-up:
	$(MIGRATE) -path migrations -database "$(DB_DSN)" up

## migrate-down: откатить все миграции
migrate-down:
	$(MIGRATE) -path migrations -database "$(DB_DSN)" down

## seed: наполнить БД демо-данными
seed:
	go run ./cmd/seed

## run: запустить chat-service (gRPC :50051 + веб-клиент на http://localhost:8080)
run:
	go run ./cmd/chat-service

## web: открыть веб-клиент в браузере (сервер должен быть запущен через make run)
web:
	open http://localhost:8080

## lint: статический анализ
lint:
	$(GOLANGCILINT) run

## test: юнит- и интеграционные тесты
test:
	go test ./...
