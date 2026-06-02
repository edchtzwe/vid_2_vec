package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/lib/pq"

	"ingestion/internal/config"
	"ingestion/migrations"
)

func main() {
	// 1. Parse Flags
	upCmd := flag.Bool("up", false, "Apply all pending migrations (Default)")
	downCmd := flag.Bool("down", false, "Rollback exactly one migration step")
	flag.Parse()

	// Default to UP if nothing specified
	if !*upCmd && !*downCmd {
		*upCmd = true
	}

	if *upCmd && *downCmd {
		log.Fatal("Error: Cannot run both --up and --down simultaneously.")
	}

	// 2. Load Config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	log.Printf("Target Schema: %s | Migration History: %s.%s",
		cfg.DBSchema, cfg.DBMigrationSchema, cfg.DBMigrationTable)

	// 3. Connect to DB
	separator := "?"
	if strings.Contains(cfg.DatabaseURL, "?") {
		separator = "&"
	}
	// We set search_path to the TARGET schema so your SQL files apply there
	dsn := fmt.Sprintf("%s%ssearch_path=%s&sslmode=disable",
		cfg.DatabaseURL, separator, cfg.DBSchema)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}

	// 4. Ensure Schemas Exist
	_, err = db.Exec(fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", cfg.DBSchema))
	if err != nil {
		log.Fatalf("failed to create target schema: %v", err)
	}
	_, err = db.Exec(fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", cfg.DBMigrationSchema))
	if err != nil {
		log.Fatalf("failed to create migration schema: %v", err)
	}

	// 5. Configure Driver (History goes to DBMigrationSchema/Public)
	driver, err := postgres.WithInstance(db, &postgres.Config{
		MigrationsTable: cfg.DBMigrationTable,
		SchemaName:      cfg.DBMigrationSchema,
	})
	if err != nil {
		log.Fatalf("driver error: %v", err)
	}

	// 6. Load Source
	source, err := iofs.New(migrations.MigrationsFS, ".")
	if err != nil {
		log.Fatalf("source error: %v", err)
	}

	// 7. Init Migrate
	m, err := migrate.NewWithInstance("iofs", source, "postgres", driver)
	if err != nil {
		log.Fatalf("migrate instance error: %v", err)
	}

	// 8. Execute Logic
	if *upCmd {
		log.Println("Running UP (All Steps)...")
		if err := m.Up(); err != nil {
			if err == migrate.ErrNoChange {
				log.Println("Database is already up to date.")
			} else {
				log.Fatalf("Migration UP failed: %v", err)
			}
		} else {
			log.Println("Migration UP successful!")
		}
	} else if *downCmd {
		log.Println("Running DOWN (1 Step)...")
		if err := m.Steps(-1); err != nil {
			if err == migrate.ErrNoChange {
				log.Println("No migrations to rollback.")
			} else if _, ok := err.(migrate.ErrShortLimit); ok {
				log.Println("No migrations to rollback (start of history).")
			} else {
				log.Fatalf("Migration DOWN failed: %v", err)
			}
		} else {
			log.Println("Migration DOWN successful!")
		}
	}
}
