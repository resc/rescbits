package migrations

// generate the in-memory scripts filesystem
//go:generate statik -f -src ./scripts


import (
	_ "github.com/resc/rescbits/bitbot/migrations/statik"
	"github.com/resc/statik/fs"
	"github.com/pkg/errors"

	"database/sql"
	_ "github.com/lib/pq"

	"time"
	"regexp"
	"strconv"
	"io/ioutil"
	"sort"
	"fmt"
)

type (
	migrations struct {
		fs               fs.FileSystem
		connectionString string
	}

	MigrationRunner interface {
		Ping() error
		IsUpToDate() (bool, error)
		GetMigrationStatus() (Migrations, error)
		Migrate() error
	}

	Migration struct {
		Id        int64
		Type      string
		Name      string
		Script    string
		IsApplied bool
		AppliedOn time.Time
	}

	Migrations []*Migration

	FileSystem interface {
		Open(name string) ([]byte, error)
		Files() []string
	}
)

func (mm Migrations) IndexOfId(id int64) int {
	for i := range mm {
		if mm[i].Id == id {
			return i
		}
	}
	return -1
}

func (mm Migrations) Len() int {
	return len(mm)
}

func (mm Migrations) Less(i, j int) bool {
	return mm[i].Id < mm[j].Id
}

func (mm Migrations) Swap(i, j int) {
	mm[j], mm[i] = mm[i], mm[j]
}

var _ sort.Interface = Migrations(nil)

const (
	queryCurrentDate           = "SELECT NOW() AS currentdate;"
	queryMigrationsTableExists = "SELECT to_regclass('public.Migrations') IS NOT NULL;"
	queryAllMigrations         = "SELECT Id, AppliedOn, Name FROM Migrations ORDER BY Id"
	queryInsertMigration       = "INSERT INTO Migrations (Id, AppliedOn, Name) VALUES ($1, $2, $3)"
)

func New(connectionString string) (MigrationRunner, error) {
	scriptFileSystem, err := fs.New()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to load migration scripts")
	}

	m := &migrations{
		connectionString: connectionString,
		fs:               scriptFileSystem,
	}

	return m, nil
}

func (m *migrations) IsUpToDate() (bool, error) {
	if mm, err := m.GetMigrationStatus(); err != nil {
		return false, errors.Wrap(err, "Failed to load migration status")
	} else {
		for i := range mm {
			if !mm[i].IsApplied {
				return false, nil
			}
		}
		return true, nil
	}
}

func (m migrations) Migrate() error {
	db, err := m.openDatabase()
	if err != nil {
		return errors.Wrap(err, "Could not open database connection")
	}
	mm, err := m.GetMigrationStatus()
	if err != nil {
		return errors.Wrap(err, "Could not fetch migration status")
	}
	for i := range mm {
		if mm[i].IsApplied {
			continue
		}

		tx, err := db.Begin()
		if err != nil {
			return errors.Wrapf(err, "Error starting transaction for migration %d %s", mm[i].Id, mm[i].Name)
		}

		// run the migration in a inline func to ensure a rollback if the migration panics
		err = func() (returnErr error) {
			defer func() {
				err, ok := recover().(error)
				if ok && err != nil {
					returnErr = err
					tx.Rollback()
				}
			}()

			// execute the migration
			if _, err := tx.Exec(mm[i].Script); err != nil {
				panic(err)
			}

			// update the administration
			appliedOn := time.Now()
			if _, err := tx.Exec(queryInsertMigration, mm[i].Id, appliedOn, mm[i].Name); err != nil {
				panic(err)
			}

			// commit the changes
			if err := tx.Commit(); err != nil {
				panic(err)
			}

			mm[i].IsApplied = true
			mm[i].AppliedOn = appliedOn
			return nil
		}()

		if err != nil {
			return errors.Wrapf(err, "Error executing migration %d %s", mm[i].Id, mm[i].Name)
		}
	}
	return nil
}

func (m *migrations) Ping() error {
	db, err := m.openDatabase()
	if err != nil {
		return errors.Wrap(err, "Could not open database connection")
	}
	defer db.Close()
	return db.Ping()
}

var (
	migrationPattern *regexp.Regexp = regexp.MustCompile(`^.*/(?P<type>[A-Z])(?P<id>\d+)_(?P<name>.*)\.[sS][qQ][lL]$`)
)

func parseScriptName(input string) (map[string]string) {
	match := migrationPattern.FindStringSubmatch(input)
	subMatches := make(map[string]string)
	if len(match) == 0 {
		return subMatches
	}

	for i, name := range migrationPattern.SubexpNames() {
		if i != 0 && len(name) > 0 {
			subMatches[name] = match[i]
		}
	}

	return subMatches
}

// getMigrations returns a sorted list of migrations, the status fields are not set yet.
func (m *migrations) getMigrations() (Migrations, error) {
	names := m.fs.Files()
	mm := make(map[int64]*Migration)
	for _, name := range names {
		fields := parseScriptName(name)
		if len(fields) == 0 {
			return nil, errors.Errorf("Invalid migration name '%s' should be like /path/to/M023_MyMigration.sql", name)
		}

		id, err := strconv.ParseInt(fields["id"], 10, 64)
		if err != nil {
			return nil, errors.Wrapf(err, "Invalid migration id '%s' for '%s'", fields["id"], name)
		}

		if m, exists := mm[id]; exists {
			return nil, errors.Errorf("Duplicate migration ids: '%s' and '%s' have the same id", m.Name, fields["name"])
		}

		contents, err := m.loadScript(name)
		if err != nil {
			return nil, errors.Wrapf(err, "Error loading script '%s'", name)
		}

		mm[id] = &Migration{
			Id:     id,
			Type:   fields["type"],
			Name:   fields["name"],
			Script: string(contents),
		}
	}
	result := getSortedMigrations(mm)
	return result, nil
}

func getSortedMigrations(mm map[int64]*Migration) Migrations {
	result := make(Migrations, 0, len(mm))
	for _, m := range mm {
		result = append(result, m)
	}
	sort.Stable(result)
	return result
}

func (m *migrations) loadScript(name string) ([]byte, error) {
	file, err := m.fs.Open(name)
	if err != nil {
		return nil, err
	}
	contents, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}
	return contents, nil
}

func (m *migrations) GetMigrationStatus() (Migrations, error) {
	if mm, err := m.getMigrations(); err != nil {
		return nil, errors.Wrap(err, "Failed to load migration scripts")
	} else {
		db, err := m.openDatabase()
		if err != nil {
			return nil, errors.Wrap(err, "Could not open database connection")
		}
		defer db.Close()

		migrationsTableExists := false
		if err := db.QueryRow(queryMigrationsTableExists).Scan(&migrationsTableExists); err != nil {
			return nil, errors.Wrap(err, "Could not check for migrations table")
		} else {
			// the first migration script is the migrations table creation script.
			if !migrationsTableExists {
				if mm[0].Id != 0 || mm[0].Name != "AddMigrationsTable" {
					panic(errors.New(fmt.Sprintf("The first migration should be the migrations table migration")))
				}
				if _, err := db.Exec(mm[0].Script); err != nil {
					return nil, errors.Wrap(err, "Error initializing migrations table")
				} else {
					db.Exec(queryInsertMigration, mm[0].Id, time.Now(), mm[0].Name)
				}
			}
		}

		rows, err := db.Query(queryAllMigrations)
		defer rows.Close()
		if err != nil {
			return nil, errors.Wrap(err, "Error executing migration status query")
		}

		current := &Migration{}
		for rows.Next() {
			if err := current.Scan(rows); err != nil {
				return nil, errors.Wrap(err, "Error loading migration status ")
			}
			if i := mm.IndexOfId(current.Id); i >= 0 {
				mm[i].IsApplied = true
				mm[i].AppliedOn = current.AppliedOn
			}
		}
		if err := rows.Err(); err != nil {
			return nil, errors.Wrap(err, "Error loading migration status")
		}
		return mm, nil
	}
}

func (m *migrations) openDatabase() (*sql.DB, error) {
	return sql.Open("postgres", m.connectionString)
}

// Scan scans a row like: Id int64, AppliedOn time.Time, Name string
func (m *Migration) Scan(rows *sql.Rows) error {
	return rows.Scan(&m.Id, &m.AppliedOn, &m.Name)
}

