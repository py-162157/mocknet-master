package etcd

import (
	"context"
	"strconv"
	"strings"
	"time"

	"mocknet/plugins/kubernetes"
	"mocknet/plugins/server/rpctest"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
	Kubernetes *kubernetes.Plugin
	PodToHost  map[string]string
	VxlanVni   int
	InfToVni   map[string]int

	Log logging.PluginLogger
}

func (p *Plugin) Init() error {
	if p.Deps.Log == nil {
		p.Deps.Log = logging.ForPlugin(p.String())
	}

	p.K8sNamespace = "default"
	p.VxlanVni = 0
	p.InfToVni = make(map[string]int)
	p.PodToHost = make(map[string]string)

	/*go func() {
		// prepare etcd binary file, make sure the relative position of etcd.sh
		cmd := exec.Command("bash", "./../scripts/etcd.sh") // 以当前命令行路径为准，当前路径为contiv-agent
		output, err := cmd.StdoutPipe()
		if err != nil {
			p.Log.Errorln(err)
			panic(err)
		}

		if err := cmd.Start(); err != nil {
			p.Log.Errorln(err)
			panic(err)
		} else {
			p.Log.Infoln("successfully setup and start etcd!")
		}

		reader := bufio.NewReader(output)

		var contentArray = make([]string, 0, 5)
		var index int
		contentArray = contentArray[0:0]

		for {
			line, err2 := reader.ReadString('\n')
			if err2 != nil || io.EOF == err2 {
				break
			}
			p.Log.Infoln(line)
			index++
			contentArray = append(contentArray, line)
		}
	}()*/

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

	return nil
}

func (p *Plugin) String() string {
	return "etcd"
}

// clear all key-value in etcd
func (p *Plugin) Close() error {
	kvs := clientv3.NewKV(p.EtcdClient)
	_, err := kvs.Delete(context.Background(), "/", clientv3.WithPrefix())
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
	txn := kvc.Txn(ctx)

	ops := []clientv3.Op{}

	for _, link := range links {
		node1 := link.Node1.Name
		node2 := link.Node2.Name
		intf1 := link.Node1Inf
		intf2 := link.Node2Inf
		if p.get_host(node1, intf1) != p.get_host(node2, intf2) {
			p.InfToVni[node1+"-"+intf1] = p.VxlanVni
			p.InfToVni[node2+"-"+intf2] = p.VxlanVni
			p.VxlanVni += 1
		} else {
			// -1 mean no need to vxlan connect
			p.InfToVni[node1+"-"+intf1] = -1
			p.InfToVni[node2+"-"+intf2] = -1
		}
	}

	for _, link := range links {
		value := ""
		value = value + "node1:" + link.Node1.Name + ","
		value = value + "node2:" + link.Node2.Name + ","
		value = value + "node1_inf:" + link.Node1Inf + ","
		value = value + "node2_inf:" + link.Node2Inf + ","
		value = value + "vni:" + strconv.Itoa(p.InfToVni[link.Node1.Name+"-"+link.Node1Inf])

		ops = append(ops, clientv3.OpPut("/mocknet/link/"+link.Name, value))
	}

	txn.Then(ops...)

	_, err := txn.Commit()
	if err != nil {
		p.Log.Errorln(err)
		panic(err)
	} else {
		p.Log.Infoln("successfully commit link data to master etcd")
	}

	p.wait_for_response("ParseTopologyInfo")

	return nil
}

func (p *Plugin) Send_Ready(count int) error {
	// when worker node read "yes", then the transport has done

	kv := clientv3.NewKV(p.EtcdClient)
	_, err := kv.Put(context.Background(), "/mocknet/topo/ready", strconv.Itoa(count))
	if err != nil {
		p.Log.Errorln(err)
		panic(err)
	} else {
		p.Log.Infoln("successfully send ready signal")
	}

	p.wait_for_response("NetCreationFinished")

	return nil
}

func (p *Plugin) Send_Pods_Info() (map[string]string, map[string]string) {
	// key: completed name, value: simplified name
	pod_name_reflector := make(map[string]string)
	// key: simplified name, value: completed name
	pod_name_reflector_rev := make(map[string]string)
	podlist, err := p.Kubernetes.ClientSet.CoreV1().Pods(p.K8sNamespace).List(metav1.ListOptions{})
	if err != nil {
		p.Log.Errorln(err)
		panic(err)
	}

	pods := podlist.Items

	kvs := clientv3.NewKV(p.EtcdClient)
	txn := kvs.Txn(context.Background())
	ops := []clientv3.Op{}

	for _, pod := range pods {
		cp := strings.Split(pod.Status.PodIP, ".") // conrol plane ip
		data_plane_ip := "10.1." + cp[2] + "." + cp[3]

		name := parse_pod_name(pod.Name)
		pod_name_reflector[pod.Name] = name
		pod_name_reflector_rev[name] = pod.Name
		key := "/mocknet/pods/" + name
		value := ""
		value = value + "name:" + name + ","
		value = value + "namespace:" + pod.Namespace + ","
		value = value + "podip:" + data_plane_ip + ","
		value = value + "hostip:" + pod.Status.HostIP + ","
		value = value + "hostname:" + p.Kubernetes.Nodeinfos[pod.Status.HostIP].Name

		ops = append(ops, clientv3.OpPut(key, value))

		p.PodToHost[name] = p.Kubernetes.Nodeinfos[pod.Status.HostIP].Name
	}

	txn.Then(ops...)

	_, err = txn.Commit()
	if err != nil {
		p.Log.Errorln(err)
		panic(err)
	} else {
		p.Log.Infoln("successfully commit pods data to master etcd")
	}

	for _, pod := range p.Kubernetes.MocknetTopology.Pods {
		for _, intf := range pod.Infs {
			key := pod.Name + "-" + intf
			value := p.PodToHost[pod.Name]
			p.Kubernetes.IntfToHost[key] = value
		}
	}

	p.wait_for_response("ParsePodsInfo")

	return pod_name_reflector, pod_name_reflector_rev
}

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
		if done_count == p.Kubernetes.AssignedWorkerNumber {
			p.Log.Infoln("all workers have finished ", event)
			break
		}
	}
	return nil
}

func parse_pod_name(logic_name string) string {
	split_name := strings.Split(logic_name, "-")
	return split_name[1]
}

/*func (p *Plugin) Pod_Tap_Create(pod_names []string) error {
	kvs := clientv3.NewKV(p.EtcdClient)
	for _, pod_name := range pod_names {
		if string([]byte(pod_name)[:1]) != "s" {
			key := "/vnf-agent/mocknet-pod-" + parse_pod_name(pod_name) + "/config/vpp/v2/interfaces/tap0"
			value := "{\"name\":\"tap0\",\"type\":\"TAP\",\"enabled\":true, \"vrf\":0, \"tap\":{\"version\":2,\"rx_ring_size\":256, \"enable_gso\":true, \"host_if_name\":\"tap0\"}}"
			kvs.Put(context.Background(), key, value)
		}
	}
	kvs.Put(context.Background(), "/mocknet/tap/CreationBegin", "true")
	return nil
}*/

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
