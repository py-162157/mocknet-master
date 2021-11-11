package server

// DefaultPlugin is a default instance of Controller.
var DefaultPlugin = *NewPlugin()

func NewPlugin(opts ...Option) *Plugin {
	p := &Plugin{}
	p.PluginName = "server"

	for _, o := range opts {
		o(p)
	}

	return p
}

// Option is a function that acts on a Plugin to inject Dependencies or configuration
type Option func(*Plugin)

// UseDeps returns Option that can inject custom dependencies.
func UseDeps(cb func(*Deps)) Option {
	return func(p *Plugin) {
		cb(&p.Deps)
	}
}
