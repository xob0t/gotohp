package protocol

import (
	"encoding/json"
	"testing"
)

func TestRequestDecodesUpload(t *testing.T) {
	var r Request
	if err := json.Unmarshal([]byte(`{"jsonrpc":"2.0","id":"7","method":"upload","params":{"paths":["a.jpg"],"options":{"Threads":2}}}`), &r); err != nil {
		t.Fatal(err)
	}
	if r.ID != "7" || r.Method != "upload" || len(r.Params.Paths) != 1 || r.Params.Options.Threads != 2 {
		t.Fatalf("unexpected request: %+v", r)
	}
}
func TestMessageOmitsEmptyFields(t *testing.T) {
	b, err := json.Marshal(Message{JSONRPC: "2.0", ID: "1", Result: map[string]any{"ok": true}})
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"jsonrpc":"2.0","id":"1","result":{"ok":true}}` {
		t.Fatalf("%s", b)
	}
}
