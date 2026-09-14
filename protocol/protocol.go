package protocol

import "app/core"

type Request struct {
	JSONRPC string `json:"jsonrpc"`
	ID      string `json:"id"`
	Method  string `json:"method"`
	Params  struct {
		Paths   []string           `json:"paths"`
		Options core.UploadOptions `json:"options"`
	} `json:"params"`
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
