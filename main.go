package main

import (
	"os"

	"github.com/belyaev-dev/helmdoc/cmd/helmdoc"
)

func main() {
	if err := helmdoc.Execute(); err != nil {
		os.Exit(1)
	}
}
