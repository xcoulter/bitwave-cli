package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestAuthHelpHidesUnfinishedAuthenticationChoices(t *testing.T) {
	var output bytes.Buffer
	command := newAuthCmd()
	command.SetOut(&output)
	command.SetArgs([]string{"--help"})

	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	help := output.String()
	if !strings.Contains(help, "login") {
		t.Fatalf("expected browser login in auth help: %s", help)
	}
	for _, unfinished := range []string{"delegate", "resume", "agent"} {
		if strings.Contains(help, unfinished) {
			t.Fatalf("unfinished command %q appeared in auth help: %s", unfinished, help)
		}
	}
}
