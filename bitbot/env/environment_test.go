package env

import (
	"testing"
)

func TestKeyPattern(t *testing.T) {
	ensureKeyValid("THIS")
	ensureKeyValid("THIS_IS")
	ensureKeyValid("THIS_IS_A")
	ensureKeyValid("THIS_IS_A_VALID")
	ensureKeyValid("THIS_IS_A_VALID_KEY")
}
