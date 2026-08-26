package cli

import "testing"

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
