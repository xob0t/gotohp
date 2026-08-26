package cli

import (
	"io"
	"strings"
	"testing"
)

func TestIsCLIInvocation(t *testing.T) {
	cases := []struct {
		args []string
		want bool
	}{
		{nil, false},
		{[]string{"upload", "a.jpg"}, true},
		{[]string{"creds", "ls"}, true},
		{[]string{"--config", "x.config", "upload", "a.jpg"}, true},
		{[]string{"-c", "x.config", "creds", "ls"}, true},
		{[]string{"--config=x.config", "version"}, true},
		{[]string{"--help"}, true},
		{[]string{"-v"}, true},
		{[]string{"--config", "x.config"}, false},
		{[]string{"--frontend-devserver"}, false},
		{[]string{`C:\photos\a.jpg`}, false},
		{[]string{"bogus"}, false},
	}
	for _, c := range cases {
		if got := IsCLIInvocation(c.args); got != c.want {
			t.Errorf("IsCLIInvocation(%q) = %v, want %v", c.args, got, c.want)
		}
	}
}

func TestReadSecretArg(t *testing.T) {
	if got, err := readSecretArg("literal", strings.NewReader("ignored")); err != nil || got != "literal" {
		t.Fatalf("literal: got %q, %v", got, err)
	}
	if got, err := readSecretArg("-", strings.NewReader("  token-value \r\nsecond line\n")); err != nil || got != "token-value" {
		t.Fatalf("stdin: got %q, %v", got, err)
	}
	if _, err := readSecretArg("-", strings.NewReader("\n")); err == nil {
		t.Fatal("empty stdin: expected error")
	}
	if got, err := readSecretArg("-", strings.NewReader("no-trailing-newline")); err != nil || got != "no-trailing-newline" {
		t.Fatalf("eof without newline: got %q, %v", got, err)
	}
	// Must return after the first line without waiting for EOF on the reader.
	pr, pw := io.Pipe()
	go func() { _, _ = io.WriteString(pw, "first\n") }()
	if got, err := readSecretArg("-", pr); err != nil || got != "first" {
		t.Fatalf("open pipe: got %q, %v", got, err)
	}
	_ = pw.Close()
	if _, err := readSecretArg("-", strings.NewReader(strings.Repeat("x", maxSecretLen+1)+"\n")); err == nil {
		t.Fatal("overlong: expected error")
	}
}
