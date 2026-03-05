package cli

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/fatih/color"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// spinner provides a simple single-line spinner for long-running operations.
type spinner struct {
	tty    bool
	mu     sync.Mutex
	status string
	frame  int
	stopCh chan struct{}
	done   chan struct{}
}

func newSpinner(tty bool, initialStatus string) *spinner {
	return &spinner{
		tty:    tty,
		status: initialStatus,
		stopCh: make(chan struct{}),
		done:   make(chan struct{}),
	}
}

func (s *spinner) start() {
	if !s.tty {
		s.mu.Lock()
		msg := s.status
		s.mu.Unlock()
		fmt.Fprintln(os.Stderr, msg)
		go func() {
			defer close(s.done)
			<-s.stopCh
		}()
		return
	}
	go func() {
		defer close(s.done)
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-s.stopCh:
				fmt.Fprintf(os.Stderr, "\033[2K\r")
				return
			case <-ticker.C:
				s.mu.Lock()
				frame := spinnerFrames[s.frame%len(spinnerFrames)]
				status := s.status
				s.frame++
				s.mu.Unlock()
				cyan := color.New(color.FgCyan).SprintFunc()
				gray := color.New(color.FgHiBlack).SprintFunc()
				fmt.Fprintf(os.Stderr, "\033[2K\r  %s %s", cyan(frame), gray(status))
			}
		}
	}()
}

func (s *spinner) update(msg string) {
	if !s.tty {
		fmt.Fprintf(os.Stderr, "  → %s\n", msg)
		return
	}
	s.mu.Lock()
	s.status = msg
	s.mu.Unlock()
}

func (s *spinner) stop() {
	close(s.stopCh)
	<-s.done
}
