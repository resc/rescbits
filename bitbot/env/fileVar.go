package env

import (
	"os"
	"github.com/pkg/errors"
	"path/filepath"
)

type fileVar struct {
	variable
	value        string
	defaultValue string
}

var _ Var = (*fileVar)(nil)

func (v *fileVar) Get(pointer interface{}) {
	if p, ok := pointer.(*string); ok {
		*p = v.value
	} else {
		panic(errors.Errorf("Unsupported pointer type %T", pointer))
	}
}

func (v *fileVar) parse() error {
	val, present := os.LookupEnv(v.Name())
	v.isPresent = present
	if present {
		path, err := filepath.Abs(val)
		if err != nil {
			return errors.Errorf("%s variable does not represent a valid file path '%s'", v.Name(), val)
		}
		v.value = path
	} else {
		if v.IsOptional() {
			v.value = v.defaultValue
		} else {
			return errors.Errorf("missing %s variable", v.Name())
		}
	}
	return nil
}
