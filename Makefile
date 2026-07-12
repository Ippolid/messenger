# Makefile мессенджера. Цели сгруппированы по этапам; на этапе 0 работают up/down/build.

# Строка подключения к локальной БД из docker-compose
DB_DSN ?= postgres://messenger:messenger@localhost:5432/messenger?sslmode=disable
COMPOSE := docker compose -f deploy/docker-compose.yml

.PHONY: up down build proto migrate-up migrate-down seed run tui lint test

## up: поднять PostgreSQL и Redis
up:
	$(COMPOSE) up -d

## down: остановить окружение (данные в volume сохраняются)
down:
	$(COMPOSE) down

## build: собрать все бинарники
build:
	go build ./...

## proto: сгенерировать gRPC-код из proto (этап 1)
proto:
	buf generate

## migrate-up: применить миграции (этап 1)
migrate-up:
	migrate -path migrations -database "$(DB_DSN)" up

## migrate-down: откатить миграции
migrate-down:
	migrate -path migrations -database "$(DB_DSN)" down

## seed: наполнить БД демо-данными (этап 7)
seed:
	go run ./cmd/seed

## run: запустить chat-service (этап 2+)
run:
	go run ./cmd/chat-service

## tui: запустить терминальный клиент (этап 4+)
tui:
	go run ./cmd/tui

## lint: статический анализ
lint:
	golangci-lint run

## test: юнит- и интеграционные тесты
test:
	go test ./...
