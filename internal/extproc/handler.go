package extproc

import (
	"fmt"

	"github.com/labstack/echo/v4"
	"github.com/ymhiroki/pprotein-v2026/internal/collect"
)

type (
	handler struct {
		processor collect.Processor
		opts      *collect.Options
	}
)

func NewHandler(processor collect.Processor, opts *collect.Options) *handler {
	return &handler{
		processor: processor,
		opts:      opts,
	}
}

func (h *handler) Register(g *echo.Group) error {
	collector, err := collect.New(h.processor, h.opts)
	if err != nil {
		return fmt.Errorf("failed to initialize collector: %w", err)
	}

	collector.RegisterHandlers(g)
	g.GET("/:id", collector.HandleProcessed)
	return nil
}
