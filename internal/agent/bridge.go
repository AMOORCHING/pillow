package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// Bridge manages the agent subprocess and pipes its output to a parser.
type Bridge struct {
	cmd    *exec.Cmd
	parser Parser
}

// NewBridge creates a bridge for the given agent command.
// For Claude Code, it uses --print --output-format stream-json.
func NewBridge(agentName string, prompt string) (*Bridge, error) {
	switch agentName {
	case "claude":
		return newClaudeCodeBridge(prompt)
	default:
		return nil, fmt.Errorf("unsupported agent: %s (supported: claude)", agentName)
	}
}

func newClaudeCodeBridge(prompt string) (*Bridge, error) {
	args := []string{
		"--print",
		"--output-format", "stream-json",
		"--verbose",
		"--dangerously-skip-permissions",
	}
	if prompt != "" {
		args = append(args, prompt)
	}

	cmd := exec.Command("claude", args...)
	cmd.Stderr = os.Stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdout pipe: %w", err)
	}

	parser := NewClaudeCodeParser(stdout)

	return &Bridge{
		cmd:    cmd,
		parser: parser,
	}, nil
}

// Run starts the agent process and parses its output, emitting events.
// It blocks until the process exits or the context is cancelled.
func (b *Bridge) Run(ctx context.Context, events chan<- AgentEvent) error {
	if err := b.cmd.Start(); err != nil {
		return fmt.Errorf("starting agent: %w", err)
	}

	// Parse output in the current goroutine
	parseErr := b.parser.Parse(ctx, events)

	// Wait for process to exit
	waitErr := b.cmd.Wait()

	if parseErr != nil {
		return parseErr
	}
	return waitErr
}

// Signal sends a signal to the agent process.
func (b *Bridge) Signal(sig os.Signal) error {
	if b.cmd.Process == nil {
		return nil
	}
	return b.cmd.Process.Signal(sig)
}

// Pause sends SIGSTOP to the agent process.
func (b *Bridge) Pause() error {
	return b.Signal(syscall.SIGSTOP)
}

// Resume sends SIGCONT to the agent process.
func (b *Bridge) Resume() error {
	return b.Signal(syscall.SIGCONT)
}

// Kill forcefully terminates the agent process.
func (b *Bridge) Kill() error {
	if b.cmd.Process == nil {
		return nil
	}
	return b.cmd.Process.Kill()
}
