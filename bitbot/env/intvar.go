package env

import (
	"os"
	"strconv"
	"github.com/pkg/errors"
)

type intVar struct {
	variable
	value        int
	defaultValue int
}

var _ Var = (*intVar)(nil)

func (v *intVar) Get(pointer interface{}) {
	if p, ok := pointer.(*int); ok {
		*p = v.value
	} else if p, ok := pointer.(*int64); ok {
		*p = int64(v.value)
	} else if p, ok := pointer.(*int32); ok {
		*p = int32(v.value)
	} else if p, ok := pointer.(*int16); ok {
		*p = int16(v.value)
	} else if p, ok := pointer.(*int8); ok {
		*p = int8(v.value)
	} else {
		panic(errors.Errorf("Unsupported pointer type %T", pointer))
	}
}
func (v *intVar) parse() error {
	strval, present := os.LookupEnv(v.Name())
	v.isPresent = present
	if present {
		val, err := strconv.ParseInt(strval, 10, 64)
		if err != nil {
			return errors.Errorf("error parsing %s variable, expected a number like 123, got '%s'", v.Name(), strval)
		}
		v.value = int(val)
	} else {
		if v.IsOptional() {
			v.value = v.defaultValue
		} else {
			return errors.Errorf("missing %s variable, expected a number like 123", v.Name())
		}
	}
	return nil
}
