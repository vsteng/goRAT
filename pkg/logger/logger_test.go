package logger

import (
	"testing"
)

func TestLoggerInit(t *testing.T) {
	Init(InfoLevel, "text")
	log := Get()
	if log == nil {
		t.Fatal("Logger is nil")
	}
}

func TestLoggerLevels(t *testing.T) {
	Init(DebugLevel, "text")
	log := Get()
	log.Debug("debug")
	log.Info("info")
	log.Warn("warn")
	log.Error("error")
}

func TestLoggerWith(t *testing.T) {
	Init(InfoLevel, "text")
	log := Get()
	log.InfoWith("message", "key", "value")
}

func TestLoggerFormats(t *testing.T) {
	for _, fmt := range []string{"text", "json"} {
		Init(InfoLevel, fmt)
		log := Get()
		if log == nil {
			t.Errorf("Logger nil for format %s", fmt)
		}
	}
}
