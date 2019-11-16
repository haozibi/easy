package easy

import "testing"

func TestEasy(t *testing.T) {

	e := New()
	err := e.ListenAndServe(":9090")
	if err != nil {
		t.Error(err)
	}
}
