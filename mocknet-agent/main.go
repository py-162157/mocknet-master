package main

import (
	"os"
	"time"

	"mocknet/plugins/etcd"
	"mocknet/plugins/kubernetes"
	"mocknet/plugins/server"

	"go.ligato.io/cn-infra/v2/agent"
	"go.ligato.io/cn-infra/v2/logging/logmanager"
	"go.ligato.io/cn-infra/v2/logging/logrus"
)

const (
	defaultStartupTimeout = 45 * time.Second
)

type MocknetAgent struct {
	name       string
	LogManager *logmanager.Plugin

	Kubernetes *kubernetes.Plugin
	Server     *server.Plugin
	ETCD       *etcd.Plugin
}

func (ma *MocknetAgent) String() string {
	return "MocknetAgent"
}

func (ma *MocknetAgent) Init() error {
	return nil
}

func (ma *MocknetAgent) Close() error {
	return nil
}

func main() {

	LogmanagerPlugin := &logmanager.DefaultPlugin
	KubernetesPlugin := &kubernetes.DefaultPlugin

	EtcdPlugin := etcd.NewPlugin(etcd.UseDeps(func(deps *etcd.Deps) {
		deps.Kubernetes = KubernetesPlugin
	}))

	ServerPlugin := server.NewPlugin(server.UseDeps(func(deps *server.Deps) {
		deps.Kubernetes = KubernetesPlugin
		deps.ETCD = EtcdPlugin
	}))

	mocknetAgent := &MocknetAgent{
		name:       "mocknetagent",
		LogManager: LogmanagerPlugin,
		Server:     ServerPlugin,
		Kubernetes: KubernetesPlugin,
		ETCD:       EtcdPlugin,
	}

	a := agent.NewAgent(agent.AllPlugins(mocknetAgent), agent.StartTimeout(getStartupTimeout()))
	if err := a.Run(); err != nil {
		logrus.DefaultLogger().Fatal(err)
	}

}

func getStartupTimeout() time.Duration {
	var err error
	var timeout time.Duration

	// valid env value must conform to duration format
	// e.g: 45s
	envVal := os.Getenv("STARTUPTIMEOUT")

	if timeout, err = time.ParseDuration(envVal); err != nil {
		timeout = defaultStartupTimeout
	}

	return timeout
}
