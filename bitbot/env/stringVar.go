package env

import (
	"os"
	"github.com/pkg/errors"
)

type stringVar struct {
	variable
	value        string
	defaultValue string
}

var _ Var = (*stringVar)(nil)

func (v *stringVar) Get(pointer interface{}) {
	if p, ok := pointer.(*string); ok {
		*p = v.value
	} else {
		panic(errors.Errorf("Unsupported pointer type %T", pointer))
	}
}

func (v *stringVar) parse() error {
	val, present := os.LookupEnv(v.Name())
	v.isPresent = present
	v.raw = val
	if present {
		v.value = val
	} else {
		if v.IsOptional() {
			v.value = v.defaultValue
		} else {
			return errors.Errorf("missing %s variable", v.Name())
		}
	}
	return nil
}
