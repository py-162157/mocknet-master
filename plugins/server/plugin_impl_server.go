package server

import (
	"net"

	"mocknet/plugins/etcd"
	"mocknet/plugins/kubernetes"
	"mocknet/plugins/server/impl"
	"mocknet/plugins/server/rpctest"

	"go.ligato.io/cn-infra/v2/logging"

	"google.golang.org/grpc"
)

type Plugin struct {
	Deps

	PluginName string
	ListenPort string // e.g. ":10010"

	DataChannel      chan rpctest.Message
	MnNameReflector  map[string]string // key: pods name in mininet, value: pods name in k8s
	K8sNameReflector map[string]string // key: pods name in k8s, value: pods name in mininet
}

type Deps struct {
	Log        logging.PluginLogger
	Kubernetes *kubernetes.Plugin
	ETCD       *etcd.Plugin
}

func (p *Plugin) Init() error {
	p.PluginName = "server"
	p.ListenPort = ":10010" // take it as temporary test port
	p.DataChannel = make(chan rpctest.Message)
	p.K8sNameReflector = make(map[string]string)
	p.MnNameReflector = make(map[string]string)

	if p.Deps.Log == nil {
		p.Deps.Log = logging.ForPlugin(p.String())
	}

	go p.start_server()

	return nil
}

func (p *Plugin) start_server() {
	listener, err := net.Listen("tcp", p.ListenPort)
	if err != nil {
		p.Log.Fatalf("failed to listen: %v", err)
	}
	p.Log.Infoln("successfully listened port", p.ListenPort)

	server := grpc.NewServer()
	rpctest.RegisterMocknetServer(server, &impl.Server{
		DataChan: p.DataChannel,
	})

	p.Log.Infoln("successfully registered the service!")
	go server.Serve(listener)
	p.Log.Infoln("successfully served the listener!")

	p.ETCD.Send_Node_info()

	// 相当于contiv的eventloop里的新事件，TODO: 完善实现eventloop机制
	go func() {
		creation_count := 0
		for {
			message := <-p.DataChannel
			if message.Type == 0 {
				creation_count++
				p.Log.Infoln("Server receive a message")
				p.Log.Infoln("The message's type is 'emunet_creation'")
				p.Log.Infoln("Start to create pods")
				p.Kubernetes.Make_Topology(message)
				pod_names := p.Kubernetes.Create_Deployment(message)
				p.ETCD.Pod_Tap_Create(pod_names)
				p.Kubernetes.Pod_Tap_Config()
				p.ETCD.Send_Pods_Info()
				p.ETCD.Commit_Create_Info(message)
				p.ETCD.Send_Ready(creation_count)
			}
		}
	}()

}

func (p *Plugin) String() string {
	return "server"
}

func (p *Plugin) Close() error {
	return nil
}
