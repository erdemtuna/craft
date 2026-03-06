package ui

import (
	"bytes"
	"testing"
)

func TestProgressTTYMode(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgressWriter(&buf, true)

	p.Start("Resolving dependencies...")
	if buf.Len() == 0 {
		t.Error("expected output in TTY mode, got none")
	}
	if got := buf.String(); got != "\r\033[KResolving dependencies..." {
		t.Errorf("unexpected Start output: %q", got)
	}

	buf.Reset()
	p.Update("Fetching github.com/org/repo...")
	if got := buf.String(); got != "\r\033[KFetching github.com/org/repo..." {
		t.Errorf("unexpected Update output: %q", got)
	}

	buf.Reset()
	p.UpdateCount("Fetching dependency", 2, 5)
	if got := buf.String(); got != "\r\033[KFetching dependency 2/5..." {
		t.Errorf("unexpected UpdateCount output: %q", got)
	}

	buf.Reset()
	p.Done("Installed 8 skills from 3 packages")
	if got := buf.String(); got != "\r\033[KInstalled 8 skills from 3 packages\n" {
		t.Errorf("unexpected Done output: %q", got)
	}

	buf.Reset()
	p.Fail("Resolution failed")
	if got := buf.String(); got != "\r\033[KResolution failed\n" {
		t.Errorf("unexpected Fail output: %q", got)
	}
}

func TestProgressNonTTYMode(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgressWriter(&buf, false)

	p.Start("Resolving...")
	p.Update("Fetching...")
	p.UpdateCount("Fetching dependency", 1, 3)
	p.Done("Done")
	p.Fail("Error")

	if buf.Len() != 0 {
		t.Errorf("expected no output in non-TTY mode, got: %q", buf.String())
	}
}

func TestProgressIsTTY(t *testing.T) {
	tty := NewProgressWriter(nil, true)
	if !tty.IsTTY() {
		t.Error("expected IsTTY() = true")
	}

	nonTTY := NewProgressWriter(nil, false)
	if nonTTY.IsTTY() {
		t.Error("expected IsTTY() = false")
	}
}
