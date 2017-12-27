package datastore

import (
	"os"
	"github.com/pkg/errors"
	"net/url"
	"strings"
	"database/sql"
	"regexp"
	"fmt"
	"github.com/resc/rescbits/bitbot/migrations"
	"log"
)

var (
	TestDbConnStr = ""
)

func init() {
	const connstrKey = "BITBOT_TEST_DATABASE_CONNSTR"
	if connstr, ok := os.LookupEnv(connstrKey); !ok {
		panic(errors.New("missing environment var :" + connstrKey))
	} else {
		TestDbConnStr = connstr
		if connUrl, err := url.Parse(TestDbConnStr); err != nil {
			panic(errors.New("test connection string should be a postgres://... connection url"))
		} else {
			dbName := strings.Trim(connUrl.Path, "/")
			if len(dbName) == 0 || !regexp.MustCompile("^[a-zA-Z0-9_]+$").MatchString(dbName) {
				panic(errors.New(fmt.Sprintf("test connection string should contain a valid test database name, '%s' is not valid.", dbName)))
			}

			connUrl.Path = "" // remove the db name, otherwise we get an error if the database doesn't exist yet
			if db, err := sql.Open("postgres", connUrl.String()); err != nil {
				panic(errors.Wrap(err, "could not open database connection for creating test database"))
			} else {
				if _, err := db.Exec(fmt.Sprintf("SELECT pg_terminate_backend(pg_stat_activity.pid) FROM pg_stat_activity WHERE pg_stat_activity.datname = '%s'" , dbName)); err != nil {
					panic(errors.Wrapf(err, "could not drop connections to test database '%s'", dbName))
				}

				if _, err := db.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s",dbName)); err != nil {
					panic(errors.Wrapf(err, "could not drop existing test database '%s'", dbName))
				}
				if _, err := db.Exec(fmt.Sprintf("CREATE DATABASE %s", dbName)); err != nil {
					panic(errors.Wrapf(err, "could not create test database '%s'", dbName))
				}

				mr, err := migrations.New(TestDbConnStr)
				if err != nil {
					panic(errors.Wrapf(err, "could not create test database migrations for db '%s'", dbName))
				}

				err = mr.Migrate()
				if err != nil {
					panic(errors.Wrapf(err, "error running test database migrations for db '%s'", dbName))
				}

				mm,err:=mr.GetMigrationStatus()
				if err != nil {
					panic(errors.Wrapf(err, "error fetching test database migration status for db '%s'", dbName))
				}

				for i := range mm {
					log.Printf("\tid: %d, name: %40s, isApplied: %t", mm[i].Id, mm[i].Name, mm[i].IsApplied)
				}

				if len(mm)<5 {
					panic(errors.Errorf( "Missing migrations for db '%s'", dbName))

				}
			}
		}
	}
}
