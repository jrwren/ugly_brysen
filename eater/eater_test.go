package eater

import "testing"

func TestExtractJSONString(t *testing.T) {
	bighonkin := `<html><head></head><script>main.app={"hello":"world"}</script>`
	a := ExtractJSONString(bighonkin, "main.app")
	AssertEqual(t, `{"hello":"world"}`, a)
}

func AssertEqual(t *testing.T, expected, actual string) {
	if expected != actual {
		t.Fatal("expected", expected, "but got", actual)
	}
}
