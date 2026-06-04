package projectmcpref

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRunCommand(t *testing.T) {
	cfg := Config{Root: t.TempDir(), Commands: map[string][]string{
		"test": {"printf", "ok"},
	}}
	res, err := RunCommand(context.Background(), cfg, "test", 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != 0 || !strings.Contains(res.Output, "ok") {
		t.Fatalf("got %+v", res)
	}
	if res.TimedOut {
		t.Fatalf("unexpected timeout: %+v", res)
	}
}

func TestRunCommandDisabled(t *testing.T) {
	cfg := Config{Root: t.TempDir(), Commands: map[string][]string{}}
	if _, err := RunCommand(context.Background(), cfg, "test", time.Second); err == nil {
		t.Fatal("expected error for unconfigured command")
	}
}

func TestRunCommandNonZeroExit(t *testing.T) {
	cfg := Config{Root: t.TempDir(), Commands: map[string][]string{
		"test": {"false"},
	}}
	res, err := RunCommand(context.Background(), cfg, "test", 5*time.Second)
	if err != nil {
		t.Fatalf("non-zero exit should not be a Go error: %v", err)
	}
	if res.ExitCode == 0 {
		t.Fatalf("expected non-zero exit, got %+v", res)
	}
}

func TestRunCommandTimeout(t *testing.T) {
	cfg := Config{Root: t.TempDir(), Commands: map[string][]string{
		"test": {"sleep", "5"},
	}}
	res, err := RunCommand(context.Background(), cfg, "test", 50*time.Millisecond)
	if err != nil {
		t.Fatalf("timeout should be reported in result, not error: %v", err)
	}
	if !res.TimedOut {
		t.Fatalf("expected TimedOut, got %+v", res)
	}
}

func TestRunCommandOutputCap(t *testing.T) {
	// yes prints an unbounded stream; the cap must bound the captured output.
	cfg := Config{Root: t.TempDir(), Commands: map[string][]string{
		"test": {"yes"},
	}}
	res, _ := RunCommand(context.Background(), cfg, "test", 200*time.Millisecond)
	if len(res.Output) > maxOutputBytes+1024 {
		t.Fatalf("output not capped: %d bytes", len(res.Output))
	}
}
