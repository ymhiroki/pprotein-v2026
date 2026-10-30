package pprof

import (
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/google/pprof/driver"
	"github.com/labstack/echo/v4"
	"github.com/ymhiroki/pprotein-v2026/internal/collect"
)

type (
	// processor loads a profile into the pprof web UI and keeps the UI handlers per snapshot.
	// The handlers are served by serveUI from a fixed route instead of being added to the echo
	// router after startup, because echo's router must not be modified while serving requests.
	processor struct {
		// driverMu serializes driver.PProf, which writes package-level state in google/pprof.
		driverMu sync.Mutex

		mu sync.RWMutex
		// handlers maps snapshot ID -> UI path ("/", "/top", "/flamegraph", ...) -> handler.
		handlers map[string]map[string]http.Handler
	}
)

func newProcessor() *processor {
	return &processor{handlers: map[string]map[string]http.Handler{}}
}

func (p *processor) Cacheable() bool {
	return false
}

func (p *processor) Process(snapshot *collect.Snapshot) (io.ReadCloser, error) {
	registerProfileHandlers := func(args *driver.HTTPServerArgs) error {
		if args.Hostport != "0:0" {
			return fmt.Errorf("unxpected hostport: %v", args.Hostport)
		}

		p.mu.Lock()
		defer p.mu.Unlock()

		p.handlers[snapshot.ID] = args.Handlers
		return nil
	}

	bodyPath, err := snapshot.BodyPath()
	if err != nil {
		return nil, fmt.Errorf("failed to find snapshot body: %w", err)
	}

	options := &driver.Options{
		Flagset: NewFlagSet([]string{
			"-no_browser",
			"-http", "0:0",
			bodyPath,
		}),
		HTTPServer: registerProfileHandlers,
	}

	p.driverMu.Lock()
	defer p.driverMu.Unlock()

	if err := driver.PProf(options); err != nil {
		return nil, fmt.Errorf("pprof internal error: %w", err)
	}
	return nil, nil
}

// serveUI serves /:id/* with the pprof web UI handler of the snapshot.
func (p *processor) serveUI(c echo.Context) error {
	p.mu.RLock()
	handler, ok := p.handlers[c.Param("id")]["/"+c.Param("*")]
	p.mu.RUnlock()

	if !ok {
		return echo.ErrNotFound
	}
	handler.ServeHTTP(c.Response(), c.Request())
	return nil
}
