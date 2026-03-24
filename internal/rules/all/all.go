// Package all provides the single umbrella import that registers all rule categories.
package all

import (
	_ "github.com/belyaev-dev/helmdoc/internal/rules/availability"
	_ "github.com/belyaev-dev/helmdoc/internal/rules/config"
	_ "github.com/belyaev-dev/helmdoc/internal/rules/health"
	_ "github.com/belyaev-dev/helmdoc/internal/rules/images"
	_ "github.com/belyaev-dev/helmdoc/internal/rules/ingress"
	_ "github.com/belyaev-dev/helmdoc/internal/rules/network"
	_ "github.com/belyaev-dev/helmdoc/internal/rules/resources"
	_ "github.com/belyaev-dev/helmdoc/internal/rules/scaling"
	_ "github.com/belyaev-dev/helmdoc/internal/rules/security"
	_ "github.com/belyaev-dev/helmdoc/internal/rules/storage"
)
