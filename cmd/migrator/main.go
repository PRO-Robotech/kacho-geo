// Command kacho-migrator — database migrations runner for kacho-geo (schema
// kacho_geo). Separate binary from the API server (skill evgeniy §9 K.1 / AP-9);
// used by the deploy init-container before the main pod starts.
//
//	kacho-migrator up|down|status
//
// DSN: --dsn flag, else KACHO_MIGRATOR_DSN, else kacho-geo config (KACHO_GEO_*).
package main

import (
	"database/sql"
	"flag"
	"log"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
	"github.com/pressly/goose/v3"

	"github.com/PRO-Robotech/kacho-geo/internal/apps/kacho/config"
	"github.com/PRO-Robotech/kacho-geo/internal/migrations"
)

func main() {
	dsnFlag := flag.String("dsn", "", "database DSN (else KACHO_MIGRATOR_DSN, else KACHO_GEO_* config)")
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		log.Fatal("usage: kacho-migrator [--dsn <dsn>] {up|down|status}")
	}
	direction := args[0]

	dsn := resolveDSN(*dsnFlag)

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		log.Fatalf("goose dialect: %v", err)
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	var gerr error
	switch direction {
	case "up":
		gerr = goose.Up(db, ".")
	case "down":
		gerr = goose.Down(db, ".")
	case "status":
		gerr = goose.Status(db, ".")
	default:
		log.Fatalf("unknown migrate direction: %s (up|down|status)", direction)
	}
	if gerr != nil {
		log.Fatalf("migrate %s: %v", direction, gerr)
	}
}

// resolveDSN picks the DSN: --dsn flag > KACHO_MIGRATOR_DSN env > kacho-geo config.
func resolveDSN(flagDSN string) string {
	if flagDSN != "" {
		return flagDSN
	}
	if env := os.Getenv("KACHO_MIGRATOR_DSN"); env != "" {
		return env
	}
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config (for DSN): %v", err)
	}
	return cfg.MigrateDSN()
}
