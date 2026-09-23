package store

import (
	"database/sql"
	"errors"
	"fmt"
	"log"

	"github.com/golang-migrate/migrate/v4"
	pgxmigrate "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func RunMigrations(dsn, migrationsPath string) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open db for migrations: %w", err)
	}
	defer db.Close()

	driver, err := pgxmigrate.WithInstance(db, &pgxmigrate.Config{})
	if err != nil {
		return fmt.Errorf("create migrate driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance("file://"+migrationsPath, "pgx", driver)
	if err != nil {
		return fmt.Errorf("init migrate: %w", err)
	}

	log.Println("running migrations...")

	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			log.Println("migrations: nothing to apply")
		} else {
			return fmt.Errorf("run migrations: %w", err)
		}
	} else {
		version, _, verr := m.Version()
		if verr != nil {
			log.Println("migrations applied")
		} else {
			log.Printf("migrations applied, now at version %d", version)
		}
	}

	return nil
}
