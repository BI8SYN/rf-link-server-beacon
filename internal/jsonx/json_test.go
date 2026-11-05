package jsonx

import "testing"

func TestStrictJSON(t *testing.T) {
	type doc struct {
		Count uint64 `json:"count"`
	}
	for _, raw := range []string{`{"count":1,"count":2}`, `{"count":1,"extra":0}`, `{"count":true}`, `{"count":1e0}`, `{"count":1.0}`, `{}`, `{"count":null}`, `{"count":1} {}`, `{"count":9007199254740992}`, `{"count":1,"x":"\ud800"}`} {
		var d doc
		if Decode([]byte(raw), &d) == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	var d doc
	if err := Decode([]byte(`{"count":1}`), &d); err != nil || d.Count != 1 {
		t.Fatal(err)
	}
}
