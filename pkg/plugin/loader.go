package plugin

import (
	"fmt"
	"os/exec"

	"github.com/hashicorp/go-plugin"
)

// Loader manages the lifecycle of a plugin process.
type Loader struct {
	path   string
	client *plugin.Client
}

// NewLoader creates a loader for a plugin at the given path.
func NewLoader(path string) *Loader {
	return &Loader{path: path}
}

// Load spawns the plugin process and returns the Plugin interface.
func (l *Loader) Load() (Plugin, error) {
	pluginMap := map[string]plugin.Plugin{
		"flowgate_plugin": &FlowgatePluginImpl{},
	}

	l.client = plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig: Handshake,
		Plugins:         pluginMap,
		Cmd:             exec.Command(l.path),
		AllowedProtocols: []plugin.Protocol{
			plugin.ProtocolGRPC,
		},
	})

	rpcClient, err := l.client.Client()
	if err != nil {
		l.client.Kill()
		return nil, fmt.Errorf("failed to create rpc client: %w", err)
	}

	raw, err := rpcClient.Dispense("flowgate_plugin")
	if err != nil {
		l.client.Kill()
		return nil, fmt.Errorf("failed to dispense plugin: %w", err)
	}

	p, ok := raw.(Plugin)
	if !ok {
		l.client.Kill()
		return nil, fmt.Errorf("plugin does not implement the Plugin interface")
	}

	return p, nil
}

// Close kills the plugin process.
func (l *Loader) Close() {
	if l.client != nil {
		l.client.Kill()
	}
}
