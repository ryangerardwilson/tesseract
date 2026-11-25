package player

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunPrintsThePlayerHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"help"}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("code %d %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "musica terminal music player") || !strings.Contains(stdout.String(), "r2d2 musica") {
		t.Fatalf("%s", stdout.String())
	}
}
