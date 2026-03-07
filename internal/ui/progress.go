// Package ui provides TTY-aware progress output and dependency tree rendering.
package ui

import (
	"fmt"
	"io"
	"os"
	"sync"

	"golang.org/x/term"
)

// Progress provides TTY-aware status output to stderr.
// On TTY: writes status lines with \r carriage return (overwriting previous).
// On non-TTY: suppresses progress output entirely (CI-friendly).
type Progress struct {
	w     io.Writer
	isTTY bool
	mu    sync.Mutex
}

// NewProgress creates a Progress that writes to stderr.
// Progress is suppressed when stderr is not a TTY.
func NewProgress() *Progress {
	isTTY := term.IsTerminal(int(os.Stderr.Fd()))
	return &Progress{
		w:     os.Stderr,
		isTTY: isTTY,
	}
}

// NewProgressWriter creates a Progress that writes to the given writer.
// isTTY controls whether progress output is emitted.
func NewProgressWriter(w io.Writer, isTTY bool) *Progress {
	return &Progress{
		w:     w,
		isTTY: isTTY,
	}
}

// Start prints an initial status message.
func (p *Progress) Start(msg string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.isTTY {
		return
	}
	_, _ = fmt.Fprintf(p.w, "\r\033[K%s", msg)
}

// Update prints a progress update, overwriting the previous line on TTY.
func (p *Progress) Update(msg string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.isTTY {
		return
	}
	_, _ = fmt.Fprintf(p.w, "\r\033[K%s", msg)
}

// UpdateCount prints a counted progress line like "Fetching dependency 2/5...".
func (p *Progress) UpdateCount(action string, current, total int) {
	p.Update(fmt.Sprintf("%s %d/%d...", action, current, total))
}

// Done clears the progress line and prints a completion message.
func (p *Progress) Done(msg string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.isTTY {
		return
	}
	_, _ = fmt.Fprintf(p.w, "\r\033[K%s\n", msg)
}

// Fail clears the progress line and prints an error message.
func (p *Progress) Fail(msg string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.isTTY {
		return
	}
	_, _ = fmt.Fprintf(p.w, "\r\033[K%s\n", msg)
}

// IsTTY returns whether progress output is enabled.
func (p *Progress) IsTTY() bool {
	return p.isTTY
}
