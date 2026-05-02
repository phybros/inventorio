package main

import "testing"

func TestDatabaseURLFromEnvUsesDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://custom:secret@db.example.com:5432/custom?sslmode=require")

	got := databaseURLFromEnv()
	want := "postgres://custom:secret@db.example.com:5432/custom?sslmode=require"
	if got != want {
		t.Fatalf("databaseURLFromEnv() = %q, want %q", got, want)
	}
}

func TestDatabaseURLFromEnvBuildsFromDBVars(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("DB_HOST", "postgres")
	t.Setenv("DB_PORT", "6543")
	t.Setenv("DB_NAME", "inventory_prod")
	t.Setenv("DB_USER", "inv-user")
	t.Setenv("DB_PASSWORD", "p@ss/word")
	t.Setenv("DB_SSLMODE", "require")

	got := databaseURLFromEnv()
	want := "postgres://inv-user:p%40ss%2Fword@postgres:6543/inventory_prod?sslmode=require"
	if got != want {
		t.Fatalf("databaseURLFromEnv() = %q, want %q", got, want)
	}
}

func TestDatabaseURLFromEnvDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("DB_HOST", "")
	t.Setenv("DB_PORT", "")
	t.Setenv("DB_NAME", "")
	t.Setenv("DB_USER", "")
	t.Setenv("DB_PASSWORD", "")
	t.Setenv("DB_SSLMODE", "")

	got := databaseURLFromEnv()
	want := "postgres://inv:inv@localhost:5432/inventory?sslmode=disable"
	if got != want {
		t.Fatalf("databaseURLFromEnv() = %q, want %q", got, want)
	}
}
