package env

import (
	"os"
	"strconv"
	"github.com/pkg/errors"
)

type boolVar struct {
	variable
	value        bool
	defaultValue bool
}

var _ Var = (*boolVar)(nil)

func (v *boolVar) Get(pointer interface{}) {
	if p, ok := pointer.(*bool); ok {
		*p = v.value
	} else {
		panic(errors.Errorf("Unsupported pointer type %T", pointer))
	}
}

func (v *boolVar) parse() error {
	strVal, present := os.LookupEnv(v.Name())
	v.isPresent = present
	if present {
		if strVal == "" {
			strVal = "true"
		}

		val, err := strconv.ParseBool(strVal)
		if err != nil {
			return errors.Errorf("error parsing %s variable, expected true or false, got '%s'", v.Name(), strVal)
		}
		v.value = val
	} else {
		if v.IsOptional() {
			v.value = v.defaultValue
		} else {
			return errors.Errorf("missing %s variable, expected a number like 123.45", v.Name())
		}
	}
	return nil
}
