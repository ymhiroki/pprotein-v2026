package extproc

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os/exec"

	"github.com/ymhiroki/pprotein-v2026/internal/collect"
)

type (
	processor struct {
		confPath string
		args     func(configPath, bodyPath string) []string
	}
)

func (p *processor) Cacheable() bool {
	return true
}

func (p *processor) Process(snapshot *collect.Snapshot) (io.ReadCloser, error) {
	bodyPath, err := snapshot.BodyPath()
	if err != nil {
		return nil, fmt.Errorf("failed to find snapshot body: %w", err)
	}

	args := p.args(p.confPath, bodyPath)
	cmd := exec.Command(args[0], args[1:]...)

	res, err := cmd.Output()
	if err != nil {
		// Show the tool's own message (e.g. a bad regexp in alp.yml) in the UI, not only the exit status.
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
			return nil, fmt.Errorf("external process aborted: %w: %s", err, bytes.TrimSpace(exitErr.Stderr))
		}
		return nil, fmt.Errorf("external process aborted: %w", err)
	}

	return io.NopCloser(bytes.NewBuffer(res)), nil
}
