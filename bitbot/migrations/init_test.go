package migrations

import (
	"os"
	"github.com/pkg/errors"
	"net/url"
	"strings"
	"database/sql"
	"regexp"
	"fmt"
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
				if _, err := db.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", dbName)); err != nil {
					panic(errors.Wrapf(err, "could not drop existing test database '%s'", dbName))
				}
				if _, err := db.Exec(fmt.Sprintf("CREATE DATABASE %s", dbName)); err != nil {
					panic(errors.Wrapf(err, "could not create test database '%s'", dbName))
				}
			}
		}
	}
}
