package protocol

import "app/core"

type Request struct {
	JSONRPC string `json:"jsonrpc"`
	ID      string `json:"id"`
	Method  string `json:"method"`
	Params  Params `json:"params"`
}
type Params struct {
	Paths     []string           `json:"paths"`
	Options   core.UploadOptions `json:"options"`
	Account   string             `json:"account"`
	Recursive *bool              `json:"recursive"`
	Threads   int                `json:"threads"`
}
type Message struct {
	JSONRPC string `json:"jsonrpc"`
	ID      string `json:"id,omitempty"`
	Result  any    `json:"result,omitempty"`
	Error   *Error `json:"error,omitempty"`
	Method  string `json:"method,omitempty"`
	Params  any    `json:"params,omitempty"`
}
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
