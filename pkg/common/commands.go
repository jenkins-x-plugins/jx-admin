package common

import (
	"bufio"
	"io"
	"os/exec"
	"sync"

	"github.com/jenkins-x/jx/v2/pkg/util"
	"github.com/pkg/errors"
)

// RunCommandAndProcessOutput runs the command and processes the output line by line with the
// given handler
func RunCommandAndProcessOutput(cmd *exec.Cmd, lineHandler func(line string) bool) error {
	stdOut, _ := cmd.StdoutPipe()
	stdErr, _ := cmd.StderrPipe()
	err := cmd.Start()
	if err != nil {
		return errors.Wrap(err, "failed to start command")
	}

	// cmd.Wait() should be called only after we finish reading
	// from stdoutIn and stderrIn.
	// wg ensures that we finish
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		copyOutput(stdOut, lineHandler)
		wg.Done()
	}()

	copyOutput(stdErr, lineHandler)

	wg.Wait()

	err = cmd.Wait()
	if err != nil {
		return errors.Wrap(err, "failed to run command")
	}
	return nil
}

func copyOutput(r io.ReadCloser, fn func(line string) bool) {
	defer r.Close()
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if !fn(line) {
			return
		}
	}
}

// CommandRunner represents a command runner so that it can be stubbed out for testing
type CommandRunner func(*util.Command) (string, error)

// DefaultCommandRunner default runner if none is set
func DefaultCommandRunner(c *util.Command) (string, error) {
	return c.RunWithoutRetry()
}
