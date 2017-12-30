package env

import (
	"testing"
	"os"
)

func TestKeyPattern(t *testing.T) {
	ensureKeyValid("THIS")
	ensureKeyValid("THIS_IS")
	ensureKeyValid("THIS_IS_A")
	ensureKeyValid("THIS_IS_A_VALID")
	ensureKeyValid("THIS_IS_A_VALID_KEY")
}

func TestStringVar(t *testing.T) {
	clearVars()

	key := "THIS_IS_A_VALID_KEY"
	val := "THIS_IS_A_VALID_VALUE"

	os.Setenv(key, val)
	Required(key, "test var")

	err := Parse()
	if err != nil {
		t.Fatal(err)
	}
	str := ""

	if Get(key, &str); str != val {
		t.Fatalf("Expected %s: got %s", val, str)
	}
	if str = String(key); str != val {
		t.Fatalf("Expected %s: got %s", val, str)
	}
}

func clearVars() {
	lock.Lock()
	defer lock.Unlock()
	vars = make(map[string]Var)
}
