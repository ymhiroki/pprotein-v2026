package alp

import (
	_ "embed"
	"fmt"

	"github.com/labstack/echo/v4"
	"github.com/ymhiroki/pprotein-v2026/internal/collect"
	"github.com/ymhiroki/pprotein-v2026/internal/extproc"
	"github.com/ymhiroki/pprotein-v2026/internal/persistent"
	"github.com/ymhiroki/pprotein-v2026/internal/storage"
	"gopkg.in/yaml.v3"
)

type (
	handler struct {
		opts   *collect.Options
		config *persistent.Handler
	}
)

//go:embed alp.yml
var defaultConfig []byte

func NewHandler(opts *collect.Options, store storage.Storage) (*handler, error) {
	h := &handler{
		opts: opts,
	}

	config, err := persistent.New(store, "alp.yml", defaultConfig, h.sanitize)
	if err != nil {
		return nil, fmt.Errorf("failed to create targets: %w", err)
	}
	h.config = config

	return h, nil
}

func (h *handler) Register(g *echo.Group) error {
	h.config.RegisterHandlers(g.Group("/config"))

	if err := extproc.NewHandler(&processor{confPath: h.config.GetPath()}, h.opts).Register(g); err != nil {
		return fmt.Errorf("failed to register extproc handlers: %w", err)
	}
	return nil
}

func (h *handler) sanitize(raw []byte) ([]byte, error) {
	var config interface{}
	if err := yaml.Unmarshal(raw, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal: %w", err)
	}

	res, err := yaml.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal: %w", err)
	}
	return res, nil
}
