package migrations

import (
	"testing"
)

func TestMigrations(t *testing.T) {
	if mr, err := New(TestDbConnStr); err != nil {
		t.Fatal("Error creating MigrationRunner", err)
	} else {
		if err := mr.Ping(); err != nil {
			t.Fatal("Error testing connection", err)
		}

		if isUpToDate, err := mr.IsUpToDate(); err != nil {
			t.Fatal("Error checking if the database is up to date", err)
		} else {
			if !isUpToDate {
				t.Fatal("The database should be up to date")
			}
		}

		mm, err := mr.GetMigrationStatus()
		if err != nil {
			t.Fatal("Error fetching the migration status", err)
		}

		if len(mm) != 1 {
			t.Fatal("There should be only one migration to create the migrations table")
		}

		if mm[0].Id != 0 {
			t.Fatal("The only migration should have id zero")
		}

		for i := range mm {
			t.Logf("\tid: %d, name: %40s, isApplied: %t", mm[i].Id, mm[i].Name, mm[i].IsApplied)
		}

		if err := mr.Migrate(); err != nil {
			t.Fatal("migrating an up to date database should not give an error", err)
		}

	}

}
