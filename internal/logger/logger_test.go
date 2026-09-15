package logger

import (
	"bytes"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	var buf bytes.Buffer
	log := New("debug", &buf)

	log.Debug("debug message")
	log.Info("info message")
	log.Warn("warn message")
	log.Error("error message")

	output := buf.String()
	if !strings.Contains(output, "debug message") {
		t.Error("expected debug message in output")
	}
	if !strings.Contains(output, "info message") {
		t.Error("expected info message in output")
	}
	if !strings.Contains(output, "warn message") {
		t.Error("expected warn message in output")
	}
	if !strings.Contains(output, "error message") {
		t.Error("expected error message in output")
	}
}

func TestLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	log := New("warn", &buf)

	log.Debug("debug")
	log.Info("info")
	log.Warn("warn")
	log.Error("error")

	output := buf.String()
	if strings.Contains(output, "debug") {
		t.Error("debug should be filtered at warn level")
	}
	if strings.Contains(output, "info") {
		t.Error("info should be filtered at warn level")
	}
	if !strings.Contains(output, "warn") {
		t.Error("warn should appear at warn level")
	}
	if !strings.Contains(output, "error") {
		t.Error("error should appear at warn level")
	}
}

func TestRedact(t *testing.T) {
	var buf bytes.Buffer
	log := New("info", &buf)

	log.Info("user login: password=%s", "secret123")
	log.Info("user login: token=%s", "abc123")
	log.Info("user login: name=%s", "john")

	output := buf.String()
	if strings.Contains(output, "secret123") {
		t.Error("password should be redacted")
	}
	if strings.Contains(output, "abc123") {
		t.Error("token should be redacted")
	}
	if !strings.Contains(output, "john") {
		t.Error("name should not be redacted")
	}
}

func TestSetLevel(t *testing.T) {
	var buf bytes.Buffer
	log := New("error", &buf)

	log.Info("should not appear")
	if buf.Len() != 0 {
		t.Error("info should be filtered at error level")
	}

	log.SetLevel("info")
	log.Info("should appear")
	if buf.Len() == 0 {
		t.Error("info should appear after level change")
	}
}
