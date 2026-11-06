package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/babarot/gist/v2/internal/version"
)

// run runs gist and returns what it printed and its exit code.
func run(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = Run(args, &out, &errOut)
	return out.String(), errOut.String(), code
}

func setHome(t *testing.T) (stateHome string) {
	t.Helper()
	home := t.TempDir()
	stateHome = filepath.Join(home, "state")
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", stateHome)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, "cache"))
	t.Setenv(EnvLogLevel, "")
	return stateHome
}

func TestVersion(t *testing.T) {
	setHome(t)
	stdout, stderr, code := run(t, "version")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	if want := version.String() + "\n"; stdout != want {
		t.Errorf("stdout is %q, want %q", stdout, want)
	}
}

func TestNoArgumentsPrintsHelp(t *testing.T) {
	setHome(t)
	stdout, _, code := run(t)
	if code != 0 || !strings.Contains(stdout, "edit your GitHub Gists") {
		t.Errorf("exit %d, stdout %q", code, stdout)
	}
}

func TestCalledWrongly(t *testing.T) {
	for _, tt := range []struct {
		name string
		args []string
		want string
	}{
		{"unknown command", []string{"nope"}, `unknown command "nope"`},
		{"unknown flag", []string{"--nope"}, "unknown flag: --nope"},
		{"arguments to version", []string{"version", "x"}, "unknown command"},
		{"unknown debug value", []string{"--debug=loud"}, "invalid --debug=loud"},
		{"live with a command", []string{"version", "--debug=live"}, "--debug=live takes no command"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			setHome(t)
			stdout, stderr, code := run(t, tt.args...)
			if code != 2 {
				t.Errorf("exit %d, want 2", code)
			}
			if stdout != "" {
				t.Errorf("stdout is %q, want nothing", stdout)
			}
			if !strings.Contains(stderr, tt.want) {
				t.Errorf("stderr is %q, want %q", stderr, tt.want)
			}
		})
	}
}

// logLines are the log lines a run wrote, decoded.
func logLines(t *testing.T, stateHome string) []map[string]any {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(stateHome, "gist", "log", "gist.log"))
	if err != nil {
		t.Fatal(err)
	}
	var lines []map[string]any
	for line := range strings.Lines(string(b)) {
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("%q: %v", line, err)
		}
		lines = append(lines, m)
	}
	return lines
}

func TestLogsEachRun(t *testing.T) {
	state := setHome(t)
	run(t, "version")
	run(t, "version")
	lines := logLines(t, state)
	if len(lines) != 4 {
		t.Fatalf("got %d lines, want a start and an exit for each run: %v", len(lines), lines)
	}
	if lines[0]["msg"] != "start" || lines[1]["msg"] != "exit" {
		t.Errorf("lines are %v", lines)
	}
	if lines[0]["run"] == "" || lines[0]["run"] != lines[1]["run"] {
		t.Errorf("a run's lines do not share its ID: %v", lines[:2])
	}
	if lines[0]["run"] == lines[2]["run"] {
		t.Errorf("two runs share an ID: %v", lines)
	}
}

func TestLogLevel(t *testing.T) {
	for _, tt := range []struct {
		name string
		env  string
		flag string
		want int // the lines logged by a run, whose lines are at info
	}{
		{"default", "", "", 2},
		{"from the environment", "error", "", 0},
		{"the flag wins", "error", "--debug", 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			state := setHome(t)
			t.Setenv(EnvLogLevel, tt.env)
			args := []string{"version"}
			if tt.flag != "" {
				args = append(args, tt.flag)
			}
			if _, stderr, code := run(t, args...); code != 0 {
				t.Fatalf("exit %d, stderr %q", code, stderr)
			}
			if got := len(logLines(t, state)); got != tt.want {
				t.Errorf("got %d lines, want %d", got, tt.want)
			}
		})
	}
}

func TestBadLogLevelFails(t *testing.T) {
	setHome(t)
	t.Setenv(EnvLogLevel, "loud")
	stdout, stderr, code := run(t, "version")
	if code != 1 || stdout != "" || !strings.Contains(stderr, EnvLogLevel) {
		t.Errorf("exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
}

func TestRunsWithoutALog(t *testing.T) {
	state := setHome(t)
	// A file where the state directory should go
	if err := os.WriteFile(state, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := run(t, "version")
	if code != 0 || stdout == "" {
		t.Errorf("exit %d, stdout %q", code, stdout)
	}
	if !strings.Contains(stderr, "writing no log") {
		t.Errorf("stderr is %q", stderr)
	}
}

func TestAuthWithoutAToken(t *testing.T) {
	setHome(t)
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_CONFIG_DIR", t.TempDir())
	t.Setenv("GH_PATH", "")
	t.Setenv("PATH", "")
	stdout, stderr, code := run(t, "auth")
	if code != 1 || stdout != "" {
		t.Errorf("exit %d, stdout %q", code, stdout)
	}
	if !strings.Contains(stderr, "gh auth login") || !strings.Contains(stderr, "Gists read and write") {
		t.Errorf("stderr is %q", stderr)
	}
}
