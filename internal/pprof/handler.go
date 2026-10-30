package pprof

import (
	"fmt"

	"github.com/labstack/echo/v4"
	"github.com/ymhiroki/pprotein-v2026/internal/collect"
)

type (
	handler struct {
		opts      *collect.Options
		collector *collect.Collector
	}
)

func NewHandler(opts *collect.Options) *handler {
	return &handler{opts: opts}
}

// Register registers the shared collector endpoints and the pprof web UI of each
// snapshot at /:id/ (for example /:id/flamegraph).
func (h *handler) Register(g *echo.Group) error {
	p := newProcessor()

	collector, err := collect.New(p, h.opts)
	if err != nil {
		return fmt.Errorf("failed to initialize collector: %w", err)
	}
	h.collector = collector

	collector.RegisterHandlers(g)
	g.Any("/:id/*", p.serveUI)
	return nil
}

// Collector returns the collector created by Register.
func (h *handler) Collector() *collect.Collector {
	return h.collector
}
