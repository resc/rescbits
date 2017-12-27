package env

import "strings"

type aggregateErrors []error

func (ae *aggregateErrors) Append(e error) {
	*ae = append(*ae, e)
}
func (ae *aggregateErrors) OrNilIfEmpty() error {
	if len(*ae) == 0 {
		return nil
	}
	return ae
}

func (ae *aggregateErrors) Error() string {
	errs := *ae
	ee := make([]string, 0, len(errs))
	for i := range errs {
		ee = append(ee, errs[i].Error())
	}
	return strings.Join(ee, "\n")
}
