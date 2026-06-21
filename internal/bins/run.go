package bins

import (
	"bufio"
	"context"
	"io"
	"os/exec"
)

func runCmd(ctx context.Context, logger LogFunc, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	go streamToLogger(stdout, logger)
	go streamToLogger(stderr, logger)

	return cmd.Wait()
}

func streamToLogger(r io.Reader, logger LogFunc) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		logger(scanner.Text())
	}
}
