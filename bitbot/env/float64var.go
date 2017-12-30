package env

import (
	"os"
	"strconv"
	"github.com/pkg/errors"
)

type float64Var struct {
	variable
	value        float64
	defaultValue float64
}

var _ Var = (*float64Var)(nil)

func (v *float64Var) Get(pointer interface{}) {
	if p, ok := pointer.(*float64); ok {
		*p = v.value
	} else if p, ok := pointer.(*float32); ok {
		*p = float32(v.value)
	} else {
		panic(errors.Errorf("Unsupported pointer type %T", pointer))
	}
}

func (v *float64Var) parse() error {
	strVal, present := os.LookupEnv(v.Name())
	v.isPresent = present
	v.raw = strVal
	if present {
		val, err := strconv.ParseFloat(strVal, 64)
		if err != nil {
			return errors.Errorf("error parsing %s variable, expected a number like 123.45, got '%s'", v.Name(), strVal)
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
