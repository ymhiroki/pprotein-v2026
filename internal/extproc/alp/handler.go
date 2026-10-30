// Package alp summarizes nginx access logs in LTSV format with alp.
package alp

import (
	_ "embed"

	"github.com/ymhiroki/pprotein-v2026/internal/collect"
	"github.com/ymhiroki/pprotein-v2026/internal/extproc"
	"github.com/ymhiroki/pprotein-v2026/internal/storage"
)

//go:embed alp.yml
var defaultConfig []byte

func NewHandler(opts *collect.Options, store storage.Storage) (*extproc.Handler, error) {
	return extproc.NewHandler(opts, store, extproc.Command{
		ConfigName:    "alp.yml",
		DefaultConfig: defaultConfig,
		Args: func(configPath, bodyPath string) []string {
			return []string{"alp", "ltsv", "--config", configPath, "--format", "tsv", "--file", bodyPath}
		},
	})
}
