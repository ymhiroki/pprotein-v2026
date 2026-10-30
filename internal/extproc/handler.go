// Package extproc serves snapshots that are summarized by an external command such as alp or slp.
package extproc

import (
	"fmt"

	"github.com/labstack/echo/v4"
	"github.com/ymhiroki/pprotein-v2026/internal/collect"
	"github.com/ymhiroki/pprotein-v2026/internal/persistent"
	"github.com/ymhiroki/pprotein-v2026/internal/storage"
	"gopkg.in/yaml.v3"
)

type (
	// Command describes an external summarizer and its YAML config file.
	Command struct {
		// ConfigName is the file name of the config in the data directory.
		ConfigName string
		// DefaultConfig is saved when the config file does not exist yet.
		DefaultConfig []byte
		// Args returns the command line that summarizes bodyPath as TSV using the config at configPath.
		Args func(configPath, bodyPath string) []string
	}

	Handler struct {
		opts      *collect.Options
		command   Command
		config    *persistent.Handler
		collector *collect.Collector
	}
)

func NewHandler(opts *collect.Options, store storage.Storage, command Command) (*Handler, error) {
	config, err := persistent.New(store, command.ConfigName, command.DefaultConfig, sanitizeYAML)
	if err != nil {
		return nil, fmt.Errorf("failed to create config: %w", err)
	}

	return &Handler{
		opts:    opts,
		command: command,
		config:  config,
	}, nil
}

// Register registers GET/POST /config for the YAML config, the shared collector endpoints,
// and GET /:id that returns the TSV summary of a snapshot.
func (h *Handler) Register(g *echo.Group) error {
	h.config.RegisterHandlers(g.Group("/config"))

	p := &processor{confPath: h.config.GetPath(), args: h.command.Args}
	collector, err := collect.New(p, h.opts)
	if err != nil {
		return fmt.Errorf("failed to initialize collector: %w", err)
	}
	h.collector = collector

	collector.RegisterHandlers(g)
	g.GET("/:id", collector.HandleProcessed)
	return nil
}

// Collector returns the collector created by Register.
func (h *Handler) Collector() *collect.Collector {
	return h.collector
}

func sanitizeYAML(raw []byte) ([]byte, error) {
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
