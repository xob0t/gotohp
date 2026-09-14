package main

import (
	"app/core"
	"app/protocol"
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type reporter struct {
	done chan struct{}
	enc  *json.Encoder
	mu   *sync.Mutex
}

func (r reporter) send(method string, params any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	// Notifications intentionally omit an ID per JSON-RPC 2.0. The request
	// ID is retained by the final response, which terminates the operation.
	_ = r.enc.Encode(protocol.Message{JSONRPC: "2.0", Method: method, Params: params})
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
	configPath := ""
	if executable, err := os.Executable(); err == nil {
		configPath = filepath.Join(filepath.Dir(executable), "gotohp.config")
	}
	if err := core.LoadConfig(configPath); err != nil {
		fmt.Fprintln(os.Stderr, "failed to load config:", err)
	}
	enc := json.NewEncoder(os.Stdout)
	scan := bufio.NewScanner(os.Stdin)
	scan.Buffer(make([]byte, 64*1024), 16*1024*1024)
	for scan.Scan() {
		var req protocol.Request
		if err := json.Unmarshal(scan.Bytes(), &req); err != nil {
			_ = enc.Encode(protocol.Message{JSONRPC: "2.0", Error: &protocol.Error{Code: "invalid_request", Message: err.Error()}})
			continue
		}
		if req.JSONRPC != "2.0" || req.ID == "" || req.Method == "" {
			_ = enc.Encode(protocol.Message{JSONRPC: "2.0", Error: &protocol.Error{Code: "invalid_request", Message: "jsonrpc, id, and method are required"}})
			continue
		}
		if req.Method != "upload" {
			if req.Method == "accounts.list" {
				_ = enc.Encode(protocol.Message{JSONRPC: "2.0", ID: req.ID, Result: (&core.ConfigManager{}).GetAccounts()})
				continue
			}
			manager := &core.ConfigManager{}
			var result any
			var err error
			switch req.Method {
			case "credentials.add":
				err = manager.AddCredentials(req.Params.Auth)
			case "credentials.remove":
				err = manager.RemoveCredentials(req.Params.Email)
			case "credentials.select":
				manager.SetSelected(req.Params.Email)
			default:
				err = fmt.Errorf("unknown method %q", req.Method)
			}
			if err != nil {
				_ = enc.Encode(protocol.Message{JSONRPC: "2.0", ID: req.ID, Error: &protocol.Error{Code: "request_failed", Message: err.Error()}})
			} else {
				_ = enc.Encode(protocol.Message{JSONRPC: "2.0", ID: req.ID, Result: result})
			}
			continue
		}
		opts := req.Params.Options
		opts.Api.Account = req.Params.Account
		if req.Params.Recursive != nil {
			opts.Recursive = *req.Params.Recursive
		}
		if req.Params.Threads > 0 {
			opts.Threads = req.Params.Threads
		}
		rep := reporter{enc: enc, done: make(chan struct{}), mu: &sync.Mutex{}}
		m := core.NewUploadManager(rep, nil)
		m.Upload(req.Params.Paths, opts)
		<-rep.done
		_ = enc.Encode(protocol.Message{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{"ok": true}})
	}
}
