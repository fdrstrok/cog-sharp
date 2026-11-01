package main

import (
	csharpplugin "github.com/fdrstrok/cog-sharp/internal/plugin"
	"github.com/grafana/cog/pkg/plugins"
)

func main() {
	plugins.Run(plugins.LanguagePlugin(csharpplugin.New(plugins.DefaultLogger)))
}
