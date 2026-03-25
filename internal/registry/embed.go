package registry

import (
	"embed"
	"sync"
)

//go:embed charts/*.yaml
var embeddedCharts embed.FS

var (
	defaultOnce sync.Once
	defaultReg  *Registry
	defaultErr  error
)

// Default returns the process-wide embedded chart registry.
func Default() (*Registry, error) {
	defaultOnce.Do(func() {
		defaultReg, defaultErr = LoadFromFS(embeddedCharts)
	})
	return defaultReg, defaultErr
}
