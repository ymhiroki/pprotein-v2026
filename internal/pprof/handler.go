package pprof

import (
	"fmt"
	"sync"

	"github.com/labstack/echo/v4"
	"github.com/ymhiroki/pprotein-v2026/internal/collect"
)

type (
	handler struct {
		opts *collect.Options
	}
)

func NewHandler(opts *collect.Options) *handler {
	return &handler{opts: opts}
}

func (h *handler) Register(g *echo.Group) error {
	p := &processor{mu: &sync.Mutex{}, route: g}

	collector, err := collect.New(p, h.opts)
	if err != nil {
		return fmt.Errorf("failed to initialize collector: %w", err)
	}

	collector.RegisterHandlers(g)
	return nil
}
