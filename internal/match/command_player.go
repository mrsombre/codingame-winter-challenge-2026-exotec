package match

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	engine "codingame/internal/engine"
)

const (
	initialTurnTimeout = time.Second
	turnTimeout        = 50 * time.Millisecond
)

type timeoutError struct{}

func (e *timeoutError) Error() string {
	return "timeout"
}

type commandPlayer struct {
	player     *engine.Player
	path       string
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	stdout     *bufio.Reader
	stderrDone chan struct{}
	turns      int
	writeMu    sync.Mutex
}

func newCommandPlayer(player *engine.Player, path string) (*commandPlayer, error) {
	cmd := exec.Command(path)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	cp := &commandPlayer{
		player:     player,
		path:       path,
		cmd:        cmd,
		stdin:      stdin,
		stdout:     bufio.NewReader(stdout),
		stderrDone: make(chan struct{}),
	}
	go func() {
		defer close(cp.stderrDone)
		_, _ = io.Copy(os.Stderr, stderr)
	}()
	return cp, nil
}

func (cp *commandPlayer) Execute() error {
	cp.writeMu.Lock()
	defer cp.writeMu.Unlock()

	lines := cp.player.ConsumeInputLines()
	cp.player.SetOutputs(nil)
	if len(lines) == 0 {
		return nil
	}

	if cp.cmd.Process == nil {
		if err := cp.cmd.Start(); err != nil {
			return fmt.Errorf("external player start failed (%s): %w", cp.path, err)
		}
	}

	for _, line := range lines {
		if _, err := fmt.Fprintln(cp.stdin, line); err != nil {
			return fmt.Errorf("external player write failed (%s): %w", cp.path, err)
		}
	}

	line, err := cp.readCommandLine()
	if err != nil {
		return err
	}
	cp.turns++
	cp.player.SetOutputs([]string{strings.TrimRight(line, "\r\n")})
	return nil
}

func (cp *commandPlayer) readCommandLine() (string, error) {
	timeout := turnTimeout
	if cp.turns == 0 {
		timeout = initialTurnTimeout
	}

	type readResult struct {
		line string
		err  error
	}

	resultCh := make(chan readResult, 1)
	go func() {
		line, err := cp.stdout.ReadString('\n')
		resultCh <- readResult{line: line, err: err}
	}()

	select {
	case result := <-resultCh:
		if result.err != nil {
			return "", fmt.Errorf("external player read failed (%s): %w", cp.path, result.err)
		}
		return result.line, nil
	case <-time.After(timeout):
		if cp.cmd != nil && cp.cmd.Process != nil {
			_ = cp.cmd.Process.Kill()
		}
		return "", &timeoutError{}
	}
}

func (cp *commandPlayer) Close() error {
	cp.writeMu.Lock()
	defer cp.writeMu.Unlock()

	if cp.stdin != nil {
		_ = cp.stdin.Close()
		cp.stdin = nil
	}
	if cp.cmd == nil {
		return nil
	}
	if cp.cmd.Process != nil {
		_ = cp.cmd.Process.Kill()
	}
	err := cp.cmd.Wait()
	<-cp.stderrDone
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == -1 {
		return nil
	}
	return err
}
