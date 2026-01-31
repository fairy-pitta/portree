package logging

import (
	"bytes"
	"testing"
)

func TestLevels(t *testing.T) {
	// Save and restore.
	origLevel := GetLevel()
	origOut := std.out
	defer func() {
		SetLevel(origLevel)
		std.mu.Lock()
		std.out = origOut
		std.mu.Unlock()
	}()

	var buf bytes.Buffer
	std.mu.Lock()
	std.out = &buf
	std.mu.Unlock()

	t.Run("quiet suppresses info", func(t *testing.T) {
		buf.Reset()
		SetLevel(LevelQuiet)
		Info("should not appear")
		if buf.Len() != 0 {
			t.Errorf("expected no output in quiet mode, got %q", buf.String())
		}
	})

	t.Run("quiet allows warn", func(t *testing.T) {
		buf.Reset()
		SetLevel(LevelQuiet)
		Warn("visible")
		if buf.Len() == 0 {
			t.Error("expected warn output in quiet mode")
		}
	})

	t.Run("normal shows info", func(t *testing.T) {
		buf.Reset()
		SetLevel(LevelNormal)
		Info("hello %s", "world")
		if buf.String() != "hello world\n" {
			t.Errorf("got %q", buf.String())
		}
	})

	t.Run("normal hides verbose", func(t *testing.T) {
		buf.Reset()
		SetLevel(LevelNormal)
		Verbose("hidden")
		if buf.Len() != 0 {
			t.Errorf("expected no output for verbose at normal level, got %q", buf.String())
		}
	})

	t.Run("verbose shows verbose", func(t *testing.T) {
		buf.Reset()
		SetLevel(LevelVerbose)
		Verbose("detail")
		if buf.String() != "detail\n" {
			t.Errorf("got %q", buf.String())
		}
	})

	t.Run("debug prefix", func(t *testing.T) {
		buf.Reset()
		SetLevel(LevelDebug)
		Debug("trace")
		if buf.String() != "[debug] trace\n" {
			t.Errorf("got %q", buf.String())
		}
	})

	t.Run("error always prints", func(t *testing.T) {
		buf.Reset()
		SetLevel(LevelQuiet)
		Error("fatal")
		if buf.String() != "error: fatal\n" {
			t.Errorf("got %q", buf.String())
		}
	})
}

func TestHelpers(t *testing.T) {
	SetLevel(LevelVerbose)
	if !IsVerbose() {
		t.Error("expected IsVerbose() = true")
	}
	if IsDebug() {
		t.Error("expected IsDebug() = false at verbose level")
	}
	if IsQuiet() {
		t.Error("expected IsQuiet() = false at verbose level")
	}

	SetLevel(LevelQuiet)
	if !IsQuiet() {
		t.Error("expected IsQuiet() = true")
	}

	SetLevel(LevelNormal)
}
