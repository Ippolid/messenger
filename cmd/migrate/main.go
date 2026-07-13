// Command migrate — тонкая обёртка над golang-migrate, собранная с нужными
// драйверами (postgres + file). Запускается через `go tool migrate` или
// `go run ./cmd/migrate`, поэтому CLI golang-migrate не нужно ставить в PATH.
//
// Использование:
//
//	go run ./cmd/migrate up          # применить все миграции
//	go run ./cmd/migrate down        # откатить все миграции
//	go run ./cmd/migrate version     # текущая версия схемы
//
// DSN берётся из флага -database или переменной окружения DB_DSN,
// путь к миграциям — из флага -path (по умолчанию ./migrations).
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/golang-migrate/migrate/v4"
	// Драйверы регистрируются через blank-импорт — это и делает бинарник
	// самодостаточным (в отличие от `go tool` над стандартным CLI migrate).
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	var (
		path string
		dsn  string
	)
	flag.StringVar(&path, "path", "migrations", "каталог с файлами миграций")
	flag.StringVar(&dsn, "database", os.Getenv("DB_DSN"), "строка подключения к БД (или переменная DB_DSN)")
	flag.Parse()

	cmd := flag.Arg(0)
	if cmd == "" {
		log.Fatal("укажите команду: up | down | version")
	}
	if dsn == "" {
		log.Fatal("не задан DSN: используйте -database или переменную окружения DB_DSN")
	}

	m, err := migrate.New("file://"+path, dsn)
	if err != nil {
		log.Fatalf("init migrate: %v", err)
	}
	defer func() {
		// Close возвращает ошибки source и database; при завершении их достаточно залогировать
		if srcErr, dbErr := m.Close(); srcErr != nil || dbErr != nil {
			log.Printf("close migrate: source=%v database=%v", srcErr, dbErr)
		}
	}()

	switch cmd {
	case "up":
		err = m.Up()
	case "down":
		err = m.Down()
	case "version":
		v, dirty, verr := m.Version()
		if verr != nil {
			log.Fatalf("version: %v", verr)
		}
		fmt.Printf("version=%d dirty=%t\n", v, dirty)
		return
	default:
		log.Fatalf("неизвестная команда %q: up | down | version", cmd)
	}

	// ErrNoChange означает, что применять нечего — это не ошибка
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		log.Fatalf("%s: %v", cmd, err)
	}
	log.Printf("migrate %s: ok", cmd)
}
