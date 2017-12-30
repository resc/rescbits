package env

type variable struct {
	name        string
	isOptional  bool
	isPresent   bool
	description string
	typ         string
	raw string
}

func (v *variable) Name() string {
	return v.name
}

func (v *variable) Description() string {
	return v.description
}

func (v *variable) IsOptional() bool {
	return v.isOptional
}

func (v *variable) IsPresent() bool {
	return v.isPresent
}

func (v *variable) Type() string {
	return v.typ
}

func (v *variable) Raw() string {
	return v.raw
}
