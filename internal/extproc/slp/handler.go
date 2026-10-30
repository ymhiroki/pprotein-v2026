// Package slp summarizes MySQL slow query logs with slp.
package slp

import (
	_ "embed"

	"github.com/ymhiroki/pprotein-v2026/internal/collect"
	"github.com/ymhiroki/pprotein-v2026/internal/extproc"
	"github.com/ymhiroki/pprotein-v2026/internal/storage"
)

//go:embed slp.yml
var defaultConfig []byte

func NewHandler(opts *collect.Options, store storage.Storage) (*extproc.Handler, error) {
	return extproc.NewHandler(opts, store, extproc.Command{
		ConfigName:    "slp.yml",
		DefaultConfig: defaultConfig,
		Args: func(configPath, bodyPath string) []string {
			return []string{"slp", "my", "--config", configPath, "--output", "standard", "--format", "tsv", "--file", bodyPath}
		},
	})
}
