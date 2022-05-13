package server

import (
	"context"
	"net"
	"sync"
	"time"

	"mocknet/plugins/etcd"
	"mocknet/plugins/kubernetes"
	"mocknet/plugins/server/impl"
	"mocknet/plugins/server/rpctest"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.ligato.io/cn-infra/v2/logging"

	"google.golang.org/grpc"
)

const (
	POD_CREATION_WATCH_INTERVAL = 2 * time.Second
	POD_CONFIG_RETRY_TIMES      = 5
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

	p.probe_etcd_plugins()
	p.probe_kubernetes_plugins()

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
			// message type=0: topo creation
			if message.Type == 0 {
				creation_count++
				p.Log.Infoln("Server receive a message")
				p.Log.Infoln("The message's type is 'emunet_creation'")
				p.Log.Infoln("Start to create pods")
				//p.Kubernetes.AffinityClusterPartition(message)
				assignment := p.Kubernetes.AffinityClusterPartition(message, 1)
				go p.watch_pod_creation_finished(assignment)
				p.ETCD.Directory_Create(assignment)
				p.Kubernetes.Make_Topology(message)
				if message.Command.EmunetCreation.Emunet.Type == "fat-tree" {
					p.ETCD.Send_pod_type_pair(p.Kubernetes.SenderPods, p.Kubernetes.ReceiverPods, p.Kubernetes.PodPair)
					p.ETCD.Send_Topology_Type("fat-tree")
				} else {
					p.ETCD.Send_Topology_Type("other")
				}
				p.ETCD.Commit_Create_Info(message)
				p.Kubernetes.Create_Deployment(assignment, 1, 11, 31)
				if message.Command.EmunetCreation.Emunet.Type == "fat-tree" {
					go p.ETCD.Wait_For_MAC()
				}
				//go p.watch_tap_recreation(context.Background())
			} else if message.Type == 3 {
				// message type = 3: full speed test
				p.Log.Println("receive a test command!")
				p.ETCD.Get_Receiver_Ready()
				p.ETCD.FullTest()
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

func (p *Plugin) watch_tap_recreation(ctx context.Context) error {
	p.Log.Infoln("watching tap reconfig signal")
	watchChan := p.ETCD.EtcdClient.Watch(context.Background(), "/mocknet/PodTapReCofiguration", clientv3.WithPrefix())

	for {
		select {
		case <-ctx.Done():
			return nil
		case resp := <-watchChan:
			err := resp.Err()
			if err != nil {
				return err
			}
			for _, ev := range resp.Events {
				if ev.IsCreate() {
					simplified_name := string(ev.Kv.Value)
					p.Log.Infoln("receive tap reconfig signal for pod", simplified_name)
					p.Kubernetes.PodList.Lock.Lock()
					p.Kubernetes.Pod_Tap_Config(p.Kubernetes.PodList.List[simplified_name].Pod)
					p.Kubernetes.PodList.Lock.Unlock()
					p.ETCD.Inform_Tap_Config_Finished(simplified_name)
					p.Log.Infoln("finished tap reconfig for pod", simplified_name)
				} else if ev.IsModify() {

				} else if ev.Type == 1 { // 1 present DELETE

				} else {
				}
			}
		}
	}
}

func (p *Plugin) watch_pod_creation_finished(assignment map[string]uint) {
	wg := sync.WaitGroup{}
	wg.Add(len(assignment))
	for podname, _ := range assignment {
		pod_name := podname
		go func() {
			var mocknet_pod *kubernetes.MocknetPod
			ok := false
			for {
				p.Kubernetes.PodList.Lock.Lock()
				mocknet_pod, ok = p.Kubernetes.PodList.List[pod_name]
				p.Kubernetes.PodList.Lock.Unlock()

				if p.Kubernetes.Is_Creation_Completed(pod_name) && ok {
					break
				}
				time.Sleep(1 * time.Second)
			}

			p.Log.Infoln("pod", pod_name, "is ready to be config")
			p.ETCD.Send_Pod_Info(mocknet_pod)

			wg.Done()
		}()
	}

	wg.Wait()
	p.ETCD.Send_Creation_Finish()
}

func (p *Plugin) probe_etcd_plugins() {
	for {
		if !p.ETCD.PluginInitFinished {
			p.Log.Infoln("waitting for plugin ETCD initiation finished")
			time.Sleep(time.Second)
		} else {
			break
		}
	}
}

func (p *Plugin) probe_kubernetes_plugins() {
	for {
		if !p.Kubernetes.PluginInitFinished {
			p.Log.Infoln("waitting for plugin kubernetes initiation finished")
			time.Sleep(time.Second)
		} else {
			break
		}
	}
}
