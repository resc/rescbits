package env

import (
	"regexp"
	"github.com/pkg/errors"
	"sync"
	"path/filepath"
	"os"
	"sort"
)

type (
	Var interface {
		Name() string
		Description() string
		IsOptional() bool
		IsPresent() bool
		Type() string
		Get(pointer interface{})

		parse() error
	}
)

var (
	// KeyPattern contains the validation pattern for env keys.
	// The default accepts variables like ALL_CAPS_WITH_UNDERSCORES.
	// It can be changed, but you should do so before you register any variables
	KeyPattern string         = "^[A-Z]+(_[A_Z]+)+$"
	vars       map[string]Var = make(map[string]Var)
	lock       sync.Mutex
)

func Vars() []Var {
	lock.Lock()
	defer lock.Unlock()

	// get all keys and sort them
	kk := make([]string, 0, len(vars))
	for key := range vars {
		kk = append(kk, key)
	}
	sort.Strings(kk)

	// get all vars in sorted order
	vv := make([]Var, 0, len(vars))
	for _, k := range kk {
		vv = append(vv, vars[k])
	}
	return vv
}

func String(key string) string {
	value := ""
	Get(key, &value)
	return value;
}

func Bool(key string) bool {
	value := false
	Get(key, &value)
	return value;
}

func Int(key string) int {
	value := 0
	Get(key, &value)
	return value;
}

func Float(key string) float64 {
	value := 0.0
	Get(key, &value)
	return value;
}

func File(key string) *os.File {
	value := ""
	Get(key, &value)
	file, err := os.Open(value);
	if err != nil {
		panic(errors.Wrapf(err, "Error opening file '%s' set with environment var %s", value, key))
	}
	return file
}

// Get retrieves the environment variable, panics if the variable is not registered or if it is required and not present
func Get(key string, pointer interface{}) {
	lock.Lock()
	defer lock.Unlock()

	v, ok := vars[key]
	if ! ok {
		panic(errors.Errorf("environment variable %s not registered", v.Name()))
	}
	if ! v.IsPresent() {
		if v.IsOptional() {
			v.Get(pointer)
		} else {
			panic(errors.Errorf("environment variable %s not present", v.Name()))
		}
	}
}

// MustParse panics on missing environment variables or optional varaibles that can't be parsed successfully
func MustParse() {
	if err := Parse(); err != nil {
		panic(err)
	}
}

// Parse returns an error detailing missing environment variables or optional variables that can't be parsed successfully
func Parse() error {
	lock.Lock()
	defer lock.Unlock()

	var errs aggregateErrors
	for _, v := range vars {
		if err := v.parse(); err != nil {
			errs.Append(err)
		}
	}

	return errs.OrNilIfEmpty()
}

func Optional(key, defaultValue, description string) {
	ensureKeyValid(key)

	lock.Lock()
	defer lock.Unlock()

	vars[key] = &stringVar{
		variable: variable{
			typ:         "string",
			isOptional:  true,
			name:        key,
			description: description,
			isPresent:   false,
		},
		value:        defaultValue,
		defaultValue: defaultValue,
	}
}

func OptionalBool(key string, defaultValue bool, description string) {
	ensureKeyValid(key)

	lock.Lock()
	defer lock.Unlock()

	vars[key] = &boolVar{
		variable: variable{
			typ:         "bool",
			isOptional:  true,
			name:        key,
			description: description,
			isPresent:   false,
		},
		value:        defaultValue,
		defaultValue: defaultValue,
	}
}

func OptionalInt(key string, defaultValue int, description string) {
	ensureKeyValid(key)

	lock.Lock()
	defer lock.Unlock()

	vars[key] = &intVar{
		variable: variable{
			typ:         "int",
			isOptional:  true,
			name:        key,
			description: description,
			isPresent:   false,
		},
		value:        defaultValue,
		defaultValue: defaultValue,
	}
}

func OptionalFloat(key string, defaultValue float64, description string) {
	ensureKeyValid(key)

	lock.Lock()
	defer lock.Unlock()
	vars[key] = &float64Var{
		variable: variable{
			typ:         "float",
			isOptional:  true,
			name:        key,
			description: description,
			isPresent:   false,
		},
		value:        defaultValue,
		defaultValue: defaultValue,
	}
}

func OptionalFile(key, defaultValue, description string) {
	ensureKeyValid(key)

	lock.Lock()
	defer lock.Unlock()

	defaultValue, err := filepath.Abs(defaultValue)
	if err != nil {
		panic(errors.Wrapf(err, "Invalid default file path: %s", defaultValue))
	}
	vars[key] = &fileVar{
		variable: variable{
			typ:         "float",
			isOptional:  true,
			name:        key,
			description: description,
			isPresent:   false,
		},
		value:        defaultValue,
		defaultValue: defaultValue,
	}
}

func Required(key, description string) {
	ensureKeyValid(key)
	lock.Lock()
	defer lock.Unlock()

	vars[key] = &stringVar{
		variable: variable{
			typ:         "string",
			isOptional:  false,
			name:        key,
			description: description,
			isPresent:   false,
		},
		value:        "",
		defaultValue: "",
	}
}

func RequiredBool(key string, description string) {
	ensureKeyValid(key)
	lock.Lock()
	defer lock.Unlock()

	vars[key] = &boolVar{
		variable: variable{
			typ:         "bool",
			isOptional:  false,
			name:        key,
			description: description,
			isPresent:   false,
		},
		value:        false,
		defaultValue: false,
	}
}

func RequiredInt(key string, description string) {
	ensureKeyValid(key)
	lock.Lock()
	defer lock.Unlock()

	vars[key] = &intVar{
		variable: variable{
			typ:         "int",
			isOptional:  false,
			name:        key,
			description: description,
			isPresent:   false,
		},
		value:        0,
		defaultValue: 0,
	}
}

func RequiredFloat(key string, description string) {
	ensureKeyValid(key)
	lock.Lock()
	defer lock.Unlock()

	vars[key] = &float64Var{
		variable: variable{
			typ:         "float",
			isOptional:  false,
			name:        key,
			description: description,
			isPresent:   false,
		},
		value:        0,
		defaultValue: 0,
	}
}

func RequiredFile(key string, description string) {
	ensureKeyValid(key)
	lock.Lock()
	defer lock.Unlock()
}

func ensureKeyValid(key string) {
	isMatch, err := regexp.MatchString(KeyPattern, key)

	if err != nil {
		panic(errors.Errorf("Invalid key pattern '%s'", err.Error()))
	}

	if !isMatch {
		panic(errors.Errorf("Invalid key name '%s'", key))
	}
}
