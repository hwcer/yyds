package model

import (
	"encoding/json"
	"testing"
)

func TestName(t *testing.T) {
	r := NewRole()
	b, _ := json.Marshal(r)
	t.Log(string(b))
}
