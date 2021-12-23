package server

import (
	"context"
	"net"
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
			if message.Type == 0 {
				creation_count++
				p.Log.Infoln("Server receive a message")
				p.Log.Infoln("The message's type is 'emunet_creation'")
				p.Log.Infoln("Start to create pods")
				go p.watch_pod_creation_finished()
				assignment := p.Kubernetes.AffinityClusterPartition(message)
				p.ETCD.Directory_Create(assignment)
				p.Kubernetes.Make_Topology(message)
				p.Kubernetes.Create_Deployment(assignment)
				p.ETCD.Commit_Create_Info(message)
				//p.Kubernetes.Pod_Tap_Config_All()
				go p.watch_tap_recreation(context.Background())
				//p.Pod_name_reflector, p.Pod_name_reflector_rev = p.ETCD.Send_Pods_Info()
				//p.ETCD.Send_Ready(creation_count)
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

func (p *Plugin) watch_pod_creation_finished() {
	for {
		p.Kubernetes.PodList.Lock.Lock()
		present_list := p.Kubernetes.PodList.List
		for _, mocknet_pod := range present_list {
			//p.Log.Infoln(mocknet_pod.Pod.Name, mocknet_pod.Handled)
			if !mocknet_pod.Handled {
				// host pod
				simplified_name := p.Kubernetes.Pod_name_reflector[mocknet_pod.Pod.Name]
				//p.Log.Infoln("setting for pod", simplified_name)
				go p.Set_Pod(simplified_name, present_list)
				p.Kubernetes.PodList.List[simplified_name].Handled = true
			}
		}
		p.Kubernetes.PodList.Lock.Unlock()

		time.Sleep(POD_CREATION_WATCH_INTERVAL)
	}
}

func (p *Plugin) Set_Pod(pod_name string, podlist map[string]*kubernetes.MocknetPod) {
	p.Log.Infoln("setting for pod", pod_name)
	// flag indicate whether the loop is broken for success or times over
	flag := true
	mocknet_pod := podlist[pod_name]
	for {
		if p.Kubernetes.Is_Creation_Completed(pod_name) {
			break
		}
		time.Sleep(1 * time.Second)
	}
	/*if string([]byte(pod_name)[:1]) != "s" {
		p.Log.Infoln("setting for pod", pod_name, "finished")
		p.Kubernetes.Pod_tap_create(mocknet_pod.Pod)
		count := 1
		for {
			if p.Kubernetes.Pod_Tap_Config(mocknet_pod.Pod) != nil {
				p.Kubernetes.Pod_tap_create(mocknet_pod.Pod)
			} else {
				break
			}
			if count == 1 {
				p.Log.Warningln("config for pod", pod_name, "failed, retrying")
			}
			count += 1
			if count >= POD_CONFIG_RETRY_TIMES {
				p.Log.Warningln("config for pod", pod_name, "failed and over max retry times, remark it as unhandled")
				p.Kubernetes.PodList.Lock.Lock()
				p.Kubernetes.PodList.List[pod_name].Handled = false
				p.Kubernetes.PodList.Lock.Unlock()
				flag = false
				break
			}
			time.Sleep(2 * time.Second)
		}
	}*/

	if flag {
		p.Log.Infoln("--------------- pod", pod_name, "config finished ---------------")
		p.ETCD.Send_Pod_Info(mocknet_pod)
	}
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
