package engine

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/snail-sky/starfleet/internal/config"
)

// Process represents a single managed process with its state machine.
type Process struct {
	Name   string
	Config config.ProcessConfig

	mu         sync.RWMutex
	state      State
	cmd        *exec.Cmd
	pid        int
	startTime  time.Time
	exitCode   int
	retryCount int
	lastError  error
	stopCh     chan struct{}
	doneCh     chan struct{}
	cancel     context.CancelFunc
	logger     *slog.Logger
}

// NewProcess creates a new Process from configuration.
func NewProcess(name string, cfg config.ProcessConfig) *Process {
	return &Process{
		Name:   name,
		Config: cfg,
		state:  StateStopped,
		stopCh: make(chan struct{}, 1),
		doneCh: make(chan struct{}),
		logger: slog.With("process", name),
	}
}

// State returns the current state of the process.
func (p *Process) State() State {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.state
}

// PID returns the PID of the running process, or 0 if not running.
func (p *Process) PID() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.pid
}

// Uptime returns how long the process has been running.
func (p *Process) Uptime() time.Duration {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.state != StateRunning {
		return 0
	}
	return time.Since(p.startTime)
}

// ExitCode returns the last exit code.
func (p *Process) ExitCode() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.exitCode
}

// RetryCount returns the current retry count.
func (p *Process) RetryCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.retryCount
}

// LastError returns the last error encountered.
func (p *Process) LastError() error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.lastError
}

// Start starts the process. It is safe to call multiple times.
func (p *Process) Start() error {
	p.mu.Lock()
	if p.state == StateRunning || p.state == StateStarting {
		p.mu.Unlock()
		return fmt.Errorf("process %s is already %s", p.Name, p.state)
	}
	p.state = StateStarting
	p.retryCount = 0
	p.stopCh = make(chan struct{}, 1)
	p.doneCh = make(chan struct{})
	p.mu.Unlock()

	go p.runLoop()
	return nil
}

// Stop gracefully stops the process.
func (p *Process) Stop() error {
	p.mu.Lock()
	if p.state == StateStopped || p.state == StateStopping || p.state == StateFatal {
		p.mu.Unlock()
		return nil
	}
	p.state = StateStopping
	cmd := p.cmd
	p.mu.Unlock()

	select {
	case p.stopCh <- struct{}{}:
	default:
	}

	if cmd != nil && cmd.Process != nil {
		p.logger.Info("sending stop signal", "signal", p.Config.StopSignal)
		sig := parseSignal(p.Config.StopSignal)
		if err := cmd.Process.Signal(sig); err != nil {
			p.logger.Warn("failed to send signal", "error", err)
		}

		done := make(chan struct{})
		go func() {
			_ = cmd.Wait()
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(time.Duration(p.Config.StopWaitSecs) * time.Second):
			p.logger.Warn("process did not stop in time, sending SIGKILL")
			_ = cmd.Process.Kill()
			<-done
		}
	}

	p.mu.Lock()
	p.state = StateStopped
	p.pid = 0
	p.cmd = nil
	if p.cancel != nil {
		p.cancel()
	}
	p.mu.Unlock()

	p.logger.Info("process stopped")
	return nil
}

// Restart stops and then starts the process.
func (p *Process) Restart() error {
	if err := p.Stop(); err != nil {
		return err
	}
	return p.Start()
}

// runLoop is the main process lifecycle loop with restart logic.
func (p *Process) runLoop() {
	defer close(p.doneCh)
	for {
		select {
		case <-p.stopCh:
			p.mu.Lock()
			p.state = StateStopped
			p.mu.Unlock()
			return
		default:
		}

		p.runOnce()

		p.mu.RLock()
		state := p.state
		autorestart := p.Config.Autorestart
		retries := p.retryCount
		maxRetries := p.Config.StartRetries
		exitCode := p.exitCode
		p.mu.RUnlock()

		if state == StateStopped || state == StateStopping {
			return
		}

		shouldRestart := false
		switch autorestart {
		case "true":
			shouldRestart = true
		case "unexpected":
			expected := false
			for _, ec := range p.Config.ExitCodes {
				if exitCode == ec {
					expected = true
					break
				}
			}
			shouldRestart = !expected
		default: // "false"
			p.mu.Lock()
			p.state = StateExited
			p.mu.Unlock()
			return
		}

		if !shouldRestart {
			p.mu.Lock()
			p.state = StateExited
			p.mu.Unlock()
			return
		}

		if retries >= maxRetries {
			p.logger.Error("exceeded start retries, entering FATAL state",
				"retries", retries, "max", maxRetries)
			p.mu.Lock()
			p.state = StateFatal
			p.mu.Unlock()
			return
		}

		delay := p.Config.RestartDelay
		if delay == 0 {
			delay = 5 * time.Second
		}
		backoff := time.Duration(retries) * delay
		if p.Config.BackoffLimit > 0 && backoff > p.Config.BackoffLimit {
			backoff = p.Config.BackoffLimit
		}

		p.logger.Info("restarting process after backoff", "delay", backoff, "retry", retries+1)
		p.mu.Lock()
		p.state = StateBackoff
		p.mu.Unlock()

		select {
		case <-time.After(backoff):
		case <-p.stopCh:
			p.mu.Lock()
			p.state = StateStopped
			p.mu.Unlock()
			return
		}

		p.mu.Lock()
		p.state = StateStarting
		p.retryCount++
		p.mu.Unlock()
	}
}

// runOnce starts the process and waits for it to exit.
func (p *Process) runOnce() {
	ctx, cancel := context.WithCancel(context.Background())
	p.mu.Lock()
	p.cancel = cancel
	p.mu.Unlock()

	args := splitCommand(p.Config.Command)
	if len(args) == 0 {
		p.mu.Lock()
		p.state = StateFatal
		p.lastError = fmt.Errorf("empty command")
		p.mu.Unlock()
		return
	}

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	if p.Config.Directory != "" {
		cmd.Dir = p.Config.Directory
	}

	// Set environment variables.
	cmd.Env = os.Environ()
	for k, v := range p.Config.Environment {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// Create process group for clean signal forwarding.
	setProcessGroup(cmd)

	// Set up log pipes.
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		p.logger.Error("failed to create stdout pipe", "error", err)
		p.mu.Lock()
		p.state = StateFatal
		p.lastError = err
		p.mu.Unlock()
		return
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		p.logger.Error("failed to create stderr pipe", "error", err)
		p.mu.Lock()
		p.state = StateFatal
		p.lastError = err
		p.mu.Unlock()
		return
	}

	p.logger.Info("starting process", "command", p.Config.Command)
	if err := cmd.Start(); err != nil {
		p.logger.Error("failed to start process", "error", err)
		p.mu.Lock()
		p.state = StateFatal
		p.lastError = err
		p.mu.Unlock()
		return
	}

	p.mu.Lock()
	p.cmd = cmd
	p.pid = cmd.Process.Pid
	p.startTime = time.Now()
	p.state = StateRunning
	p.mu.Unlock()

	p.logger.Info("process started", "pid", cmd.Process.Pid)

	// Start log collection.
	go collectLogs(stdoutPipe, p.Name, "stdout")
	go collectLogs(stderrPipe, p.Name, "stderr")

	// Wait for process to exit.
	err = cmd.Wait()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	p.mu.Lock()
	p.exitCode = exitCode
	p.lastError = err
	p.cmd = nil
	p.pid = 0
	p.mu.Unlock()

	p.logger.Info("process exited", "exit_code", exitCode)
}

// splitCommand splits a command string into program and arguments.
// This is a simple split by spaces, respecting quoted strings.
func splitCommand(cmd string) []string {
	var args []string
	var current string
	inQuote := false
	quoteChar := byte(0)

	for i := 0; i < len(cmd); i++ {
		c := cmd[i]
		if inQuote {
			if c == quoteChar {
				inQuote = false
			} else {
				current += string(c)
			}
		} else {
			switch c {
			case '"', '\'':
				inQuote = true
				quoteChar = c
			case ' ', '\t':
				if current != "" {
					args = append(args, current)
					current = ""
				}
			default:
				current += string(c)
			}
		}
	}
	if current != "" {
		args = append(args, current)
	}
	return args
}

