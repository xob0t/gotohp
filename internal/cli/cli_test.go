package cli

import (
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
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

func TestLegacyDisableFilterParsing(t *testing.T) {
	cases := []struct {
		name      string
		args      []string
		paths     []string
		album     string
		config    string
		recursive bool
		disable   bool
	}{
		{name: "legacy flag", args: []string{"upload", "photo.jpg", "-df"}, paths: []string{"photo.jpg"}, disable: true},
		{name: "legacy flag before command", args: []string{"-df", "upload", "photo.jpg"}, paths: []string{"photo.jpg"}, disable: true},
		{name: "long value before command", args: []string{"--album", "-df", "upload", "photo.jpg", "-df"}, paths: []string{"photo.jpg"}, album: "-df", disable: true},
		{name: "long value", args: []string{"upload", "--album", "-df", "photo.jpg"}, paths: []string{"photo.jpg"}, album: "-df"},
		{name: "short value then legacy flag", args: []string{"upload", "-a", "-df", "photo.jpg", "-df"}, paths: []string{"photo.jpg"}, album: "-df", disable: true},
		{name: "root flag value", args: []string{"--config", "-df", "upload", "photo.jpg", "-df"}, paths: []string{"photo.jpg"}, config: "-df", disable: true},
		{name: "short root flag value", args: []string{"-c", "-df", "upload", "photo.jpg", "-df"}, paths: []string{"photo.jpg"}, config: "-df", disable: true},
		{name: "separator", args: []string{"upload", "--", "-df", "photo.jpg"}, paths: []string{"-df", "photo.jpg"}},
		{name: "combined short value", args: []string{"upload", "-ra", "-df", "photo.jpg"}, paths: []string{"photo.jpg"}, album: "-df", recursive: true},
		{name: "attached short value", args: []string{"upload", "-a-df", "photo.jpg", "-df"}, paths: []string{"photo.jpg"}, album: "-df", disable: true},
		{name: "attached combined value", args: []string{"upload", "-ra=-df", "photo.jpg", "-df"}, paths: []string{"photo.jpg"}, album: "-df", recursive: true, disable: true},
		{name: "attached long value", args: []string{"upload", "--album=-df", "photo.jpg", "-df"}, paths: []string{"photo.jpg"}, album: "-df", disable: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := newRootCommand(Info{ExecutableName: "gotohp-cli"})
			upload, _, err := root.Find([]string{"upload"})
			if err != nil {
				t.Fatal(err)
			}
			called := false
			upload.RunE = func(cmd *cobra.Command, paths []string) error {
				called = true
				if !reflect.DeepEqual(paths, tc.paths) {
					t.Errorf("paths = %q, want %q", paths, tc.paths)
				}
				for name, want := range map[string]string{"album": tc.album, "config": tc.config} {
					if got, _ := cmd.Flags().GetString(name); got != want {
						t.Errorf("%s = %q, want %q", name, got, want)
					}
				}
				for name, want := range map[string]bool{"disable-filter": tc.disable, "recursive": tc.recursive, "delete": false, "force": false} {
					if got, _ := cmd.Flags().GetBool(name); got != want {
						t.Errorf("%s = %v, want %v", name, got, want)
					}
				}
				return nil
			}
			root.SetArgs(normalizeLegacyArgs(root, tc.args))
			if err := root.Execute(); err != nil {
				t.Fatal(err)
			}
			if !called {
				t.Fatal("upload command did not run")
			}
		})
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
	exact := strings.Repeat("x", maxSecretLen)
	if got, err := readSecretArg("-", strings.NewReader(exact+"\r\n")); err != nil || got != exact {
		t.Fatalf("exact limit: len=%d, %v", len(got), err)
	}
	if _, err := readSecretArg("-", strings.NewReader(exact+"x\n")); err == nil {
		t.Fatal("overlong: expected error")
	}
}
