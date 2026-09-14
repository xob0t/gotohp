package main

import (
	"app/core"
	"app/protocol"
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

type reporter struct {
	done chan struct{}
	enc  *json.Encoder
	id   string
}

func (r reporter) send(method string, params any) {
	_ = r.enc.Encode(protocol.Message{JSONRPC: "2.0", ID: r.id, Method: method, Params: params})
}
func (r reporter) UploadStart(v core.UploadBatchStart) { r.send("uploadStart", v) }
func (r reporter) UploadStop()                         { r.send("uploadStop", nil); close(r.done) }
func (r reporter) TotalBytes(v int64)                  { r.send("totalBytes", v) }
func (r reporter) TotalBytesDelta(v int64)             { r.send("totalBytesDelta", v) }
func (r reporter) Warning(v core.PreflightWarning)     { r.send("warning", v) }
func (r reporter) ThreadStatus(v core.ThreadStatus)    { r.send("threadStatus", v) }
func (r reporter) FileResult(v core.FileUploadResult)  { r.send("fileResult", v) }
func (r reporter) AlbumProgress(v core.AlbumStatus)    { r.send("albumProgress", v) }
func (r reporter) AlbumComplete(v core.AlbumStatus)    { r.send("albumComplete", v) }
func (r reporter) AlbumError(v core.AlbumError)        { r.send("albumError", v) }
func main() {
	enc := json.NewEncoder(os.Stdout)
	scan := bufio.NewScanner(os.Stdin)
	for scan.Scan() {
		var req protocol.Request
		if err := json.Unmarshal(scan.Bytes(), &req); err != nil {
			_ = enc.Encode(protocol.Message{JSONRPC: "2.0", Error: &protocol.Error{Code: "invalid_request", Message: err.Error()}})
			continue
		}
		if req.Method != "upload" {
			_ = enc.Encode(protocol.Message{JSONRPC: "2.0", ID: req.ID, Error: &protocol.Error{Code: "method_not_found", Message: fmt.Sprintf("unknown method %q", req.Method)}})
			continue
		}
		rep := reporter{enc: reqEncoder(enc), id: req.ID, done: make(chan struct{})}
		m := core.NewUploadManager(rep, nil)
		m.Upload(req.Params.Paths, req.Params.Options)
		<-rep.done
		for m.IsRunning() {
		}
		_ = enc.Encode(protocol.Message{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{"ok": true}})
	}
}
func reqEncoder(e *json.Encoder) *json.Encoder { return e }
