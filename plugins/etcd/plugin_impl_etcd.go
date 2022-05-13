package etcd

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	"mocknet/plugins/kubernetes"
	"mocknet/plugins/server/rpctest"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.ligato.io/cn-infra/v2/logging"
)

type Plugin struct {
	Deps

	PluginName   string
	K8sNamespace string
	EtcdClient   *clientv3.Client
}

type Deps struct {
	Kubernetes         *kubernetes.Plugin
	PodToHost          PodToHostSync
	VxlanVni           int
	InfToVni           map[string]int
	PluginInitFinished bool

	Log logging.PluginLogger
}

type PodToHostSync struct {
	Lock *sync.RWMutex
	List map[string]string
}

type InfToVniSync struct {
	Lock *sync.RWMutex
	List map[string]int
}

func (p *Plugin) Init() error {
	if p.Deps.Log == nil {
		p.Deps.Log = logging.ForPlugin(p.String())
	}

	p.K8sNamespace = "default"
	p.VxlanVni = 0
	p.InfToVni = make(map[string]int)
	p.PodToHost = PodToHostSync{
		Lock: &sync.RWMutex{},
		List: make(map[string]string),
	}

	if client, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"0.0.0.0:32379"},
		DialTimeout: 5 * time.Second,
	}); err != nil {
		p.Log.Errorln(err)
		panic(err)
	} else {
		p.EtcdClient = client
		p.Log.Infoln("successfully connected to master etcd!")
	}

	p.PluginInitFinished = true

	// clear the database
	_, err := p.EtcdClient.Delete(context.Background(), "/", clientv3.WithPrefix())
	if err != nil {
		panic("error clear all value in initialzing")
	}
	//_, err = kvs.Delete(context.Background(), "/mocknet/link/", clientv3.WithPrefix())

	p.Log.Infoln("clear all info finished in initialzing")

	return nil
}

func (p *Plugin) String() string {
	return "etcd"
}

// clear all key-value in etcd
func (p *Plugin) Close() error {
	kvs := clientv3.NewKV(p.EtcdClient)

	_, err := kvs.Put(context.Background(), "/mocknet/Close", "true")
	if err != nil {
		panic("error when send close signal")
	}

	p.wait_for_response("Close")

	// clear the database
	_, err = kvs.Delete(context.Background(), "/", clientv3.WithPrefix())
	if err != nil {
		panic("error when clear all value")
	}
	//_, err = kvs.Delete(context.Background(), "/mocknet/link/", clientv3.WithPrefix())

	p.Log.Infoln("clear all info finished")

	return nil
}

func (p *Plugin) Commit_Create_Info(message rpctest.Message) error {
	links := message.Command.EmunetCreation.Emunet.Links

	ctx := context.Background()
	kvc := clientv3.NewKV(p.EtcdClient)

	for _, link := range links {
		node1 := link.Node1.Name
		node2 := link.Node2.Name
		intf1 := link.Node1Inf
		intf2 := link.Node2Inf

		p.InfToVni[node1+"-"+intf1] = p.VxlanVni
		p.InfToVni[node2+"-"+intf2] = p.VxlanVni
		p.VxlanVni += 1

	}

	//p.Log.Infoln(message)

	for _, link := range links {
		value := ""
		value = value + "node1:" + link.Node1.Name + ","
		value = value + "node2:" + link.Node2.Name + ","
		value = value + "node1_inf:" + link.Node1Inf + ","
		value = value + "node2_inf:" + link.Node2Inf + ","
		value = value + "vni:" + strconv.Itoa(p.InfToVni[link.Node1.Name+"-"+link.Node1Inf])
		//p.Log.Infoln("key =", "/mocknet/link/"+link.Name, "value =", value)

		_, err := kvc.Put(ctx, "/mocknet/link/"+link.Name, value)

		if err != nil {
			p.Log.Errorln(err)
			panic(err)
		}
	}
	p.Log.Infoln("successfully commit link data to master etcd")
	p.wait_for_response("ParseTopologyInfo")

	return nil
}

func (p *Plugin) Send_Order(order string) {
	kv := clientv3.NewKV(p.EtcdClient)
	_, err := kv.Put(context.Background(), "/mocknet/order/"+order, "done")
	if err != nil {
		p.Log.Errorln(err)
		panic(err)
	} else {
		p.Log.Infoln("successfully send ", order, "order")
	}
}

func (p *Plugin) Send_Topology_Type(topo_type string) error {
	kv := clientv3.NewKV(p.EtcdClient)
	_, err := kv.Put(context.Background(), "/mocknet/topo/type", topo_type)
	if err != nil {
		p.Log.Errorln(err)
		panic(err)
	} else {
		p.Log.Infoln("successfully send topology type")
	}

	p.wait_for_response("TopologyType")

	return nil
}

func (p *Plugin) Send_Pod_Info(mocknet_pod *kubernetes.MocknetPod) error {
	p.PodToHost.Lock.Lock()
	defer p.PodToHost.Lock.Unlock()
	kvs := clientv3.NewKV(p.EtcdClient)

	cp := strings.Split(mocknet_pod.Pod.Status.PodIP, ".") // conrol plane ip
	//p.Log.Infoln("podip is", mocknet_pod.Pod.Status.PodIP)
	data_plane_ip := "10.1." + cp[2] + "." + cp[3]

	name := Parse_pod_name(mocknet_pod.Pod.Name)
	key := "/mocknet/pods/" + name
	value := ""
	value = value + "name:" + name + ","
	value = value + "namespace:" + mocknet_pod.Pod.Namespace + ","
	value = value + "podip:" + data_plane_ip + ","
	value = value + "hostip:" + mocknet_pod.Pod.Status.HostIP + ","
	value = value + "hostname:" + p.Kubernetes.Nodeinfos[mocknet_pod.Pod.Status.HostIP].Name + ","
	value = value + "restartcount:" + strconv.Itoa(int(mocknet_pod.RestartCount)) + ","
	value = value + "containerid:" + strings.Split(mocknet_pod.Pod.Status.ContainerStatuses[0].ContainerID, "//")[1]

	p.PodToHost.List[name] = p.Kubernetes.Nodeinfos[mocknet_pod.Pod.Status.HostIP].Name

	_, err := kvs.Put(context.Background(), key, value)
	if err != nil {
		p.Log.Errorln(err)
		panic(err)
	}

	for _, pod := range p.Kubernetes.MocknetTopology.Pods {
		for _, intf := range pod.Infs {
			key := pod.Name + "-" + intf
			value := p.PodToHost.List[pod.Name]
			p.Kubernetes.IntfToHost[key] = value
		}
	}

	return nil
}

// wait for all workers finishing some work
func (p *Plugin) wait_for_response(event string) error {
	p.Log.Infoln("waiting for event", event)
	kvs := clientv3.NewKV(p.EtcdClient)
	for {
		done_count := 0
		workers_resp, err := kvs.Get(context.Background(), "/mocknet/"+event, clientv3.WithPrefix())
		if err != nil {
			panic(err)
		}
		for _, worker_resp := range workers_resp.Kvs {
			if string(worker_resp.Value) == "done" {
				done_count += 1
			}
		}
		//p.Log.Infoln("AssignedWorkerNumber is", p.Kubernetes.AssignedWorkerNumber)
		//p.Log.Infoln("done_count is", done_count)
		// 1. normal situation that create deployment 2. situation that stop before deployment creation
		if done_count == p.Kubernetes.AssignedWorkerNumber && p.Kubernetes.AssignedWorkerNumber >= 1 {
			p.Log.Infoln("all workers have finished ", event)
			break
		}
		time.Sleep(time.Second)
	}
	return nil
}

func Parse_pod_name(logic_name string) string {
	split_name := strings.Split(logic_name, "-")
	return split_name[1]
}

func (p *Plugin) Send_Creation_Finish() error {
	key := "/mocknet/pods/CreationFinish"
	value := "done"
	kvs := clientv3.NewKV(p.EtcdClient)

	_, err := kvs.Put(context.Background(), key, value)
	if err != nil {
		p.Log.Errorln(err)
		panic(err)
	}

	p.Log.Infoln("inform worker that all pod are prepared to be configured")

	return nil
}

// send pods' type (sender or receiver) to database, only for fat-tree topology by now
func (p *Plugin) Send_pod_type_pair(SenderPods []string, ReceiverPods []string, TestPair map[string]string) error {
	key_prefix_type := "/mocknet/podtype/"
	key_prefix_pair := "/mocknet/podpair/"

	kvs := clientv3.NewKV(p.EtcdClient)
	for _, podname := range SenderPods {
		key := key_prefix_type + podname
		value := "name:" + podname + "," + "kind:" + "sender"
		kvs.Put(context.Background(), key, value)
	}

	for _, podname := range ReceiverPods {
		key := key_prefix_type + podname
		value := "name:" + podname + "," + "kind:" + "receiver"
		kvs.Put(context.Background(), key, value)
	}

	for pod1, pod2 := range TestPair {
		key := key_prefix_pair + pod1 + "-" + pod2
		value := pod1 + "-" + pod2
		kvs.Put(context.Background(), key, value)
	}
	// p.wait_for_response("PodTypeReceiveFinished")
	return nil
}

func (p *Plugin) Get_Receiver_Ready() error {
	kvs := clientv3.NewKV(p.EtcdClient)
	key := "mocknet/command/GetReceiverReady"
	value := "true"
	kvs.Put(context.Background(), key, value)

	p.wait_for_response("ReceiverReady")

	return nil
}

func (p *Plugin) FullTest() error {
	kvs := clientv3.NewKV(p.EtcdClient)
	key := "mocknet/command/FullTest"
	value := "true"
	kvs.Put(context.Background(), key, value)

	return nil
}

func (p *Plugin) Wait_Pod_Tap_Creation() {
	p.wait_for_response("PodTapCreation")
}

func (p *Plugin) Send_Node_info() error {
	kvs := clientv3.NewKV(p.EtcdClient)
	node_infos := p.Kubernetes.Assign_VTEP()
	for key, value := range node_infos {
		kvs.Put(context.Background(), key, value)
	}
	p.Log.Infoln("commit worker node ip info and vtep info to etcd")
	return nil
}

func (p *Plugin) get_host(pod string, intf string) string {
	name := pod + "-" + intf
	return p.Kubernetes.IntfToHost[name]
}

func (p *Plugin) Inform_Finished_to_Workers(event string) error {
	p.EtcdClient.Put(context.Background(), "/mocknet/"+event, "done")
	p.Log.Infoln("informed workers that", event)
	return nil
}

func (p *Plugin) Inform_Tap_Config_Finished(podname string) error {
	p.EtcdClient.Put(context.Background(), "/mocknet/PodTapConfigFinished-"+podname, podname)
	p.Log.Infoln("informed pod", podname, "that its tap interface has been configured")
	return nil
}

func (p *Plugin) Directory_Create(assignment map[string]uint) {
	for podname, hostid := range assignment {
		key := "/mocknet/assignment/" + podname
		value := "pod:" + podname + "," + "hostid:" + "worker" + strconv.Itoa(int(hostid))
		p.EtcdClient.Put(context.Background(), key, value)
	}
	p.EtcdClient.Put(context.Background(), "/mocknet/assignment/done", "done")
	p.wait_for_response("DirectoryCreationFinished")
}

func (p *Plugin) Wait_For_MAC() {
	p.wait_for_response("MACupload")
	p.Send_Order("StaticARP")
}
