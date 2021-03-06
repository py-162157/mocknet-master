package kubernetes

import (
	"bytes"
	"flag"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	affinity "mocknet/plugins/algorithm"
	"mocknet/plugins/server/rpctest"

	appsv1 "k8s.io/api/apps/v1"
	coreV1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"go.ligato.io/cn-infra/v2/logging"
	"k8s.io/client-go/kubernetes"

	//"k8s.io/client-go/kubernetes/scheme"
	k8s_rest "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	//"k8s.io/client-go/tools/remotecommand"
)

const (
	VTEP_PREFIX               = "10.2.0."
	VTEP_POSTFIX              = "/24"
	POD_STATUS_WATCH_INTERVAL = 3 * time.Second
)

type Plugin struct {
	Deps

	PluginName   string
	K8sNamespace string
	ClientSet    *kubernetes.Clientset
}

type Deps struct {
	Log             logging.PluginLogger
	KubeConfig      *k8s_rest.Config
	PodSet          map[string]map[string]string
	MocknetTopology NetTopo
	// key: pods name in mininet, value: pods name in k8s
	MnNameReflector map[string]string
	// key: pods name in k8s, value: pods name in mininet
	K8sNameReflector map[string]string
	// key: interface id in mininet(podname-intfid), value: hostname
	IntfToHost map[string]string
	// key: node_ip
	Nodeinfos              map[string]Nodeinfo
	AssignedWorkerNumber   int
	PodList                MocknetPodSync
	PluginInitFinished     bool
	SenderPods             []string
	ReceiverPods           []string
	Pod_name_reflector     map[string]string          // key: completed name, value: simplified name
	Pod_name_reflector_rev map[string]string          // key: simplified name, value: completed name
	PodCoreAssignment      map[int]*PodCoreAssignment // key: worker-name, value: present core to be allocated
	PodPair                map[string]string          // pod pair for test (sender-receiver)
}

type PodCoreAssignment struct {
	switchs CoreAssignment
	hosts   CoreAssignment
}

type CoreAssignment struct {
	start       int
	end         int
	core_string string
}

type MocknetPodSync struct {
	Lock *sync.RWMutex
	List map[string]*MocknetPod
}

type MocknetPod struct {
	Handled      bool
	RestartCount int32
	Pod          coreV1.Pod
}

type Nodeinfo struct {
	Name   string
	Nodeip string
	Vtepip string
}

type NetTopo struct {
	Pods  []Pod
	Links []Link
}

type Pod struct {
	Name string
	Infs []string
}

type Link struct {
	Name    string
	Pod1    string
	Pod2    string
	Pod1inf string
	Pod2inf string
}

func (p *Plugin) Init() error {
	if p.Deps.Log == nil {
		p.Deps.Log = logging.ForPlugin(p.String())
	}

	p.PluginName = "k8s"
	p.K8sNamespace = "default"
	p.K8sNameReflector = make(map[string]string)
	p.MnNameReflector = make(map[string]string)
	p.Nodeinfos = make(map[string]Nodeinfo)
	p.IntfToHost = make(map[string]string)
	p.PodSet = make(map[string]map[string]string)
	p.PodList = MocknetPodSync{
		Lock: &sync.RWMutex{},
		List: make(map[string]*MocknetPod),
	}
	p.Pod_name_reflector = make(map[string]string)
	p.Pod_name_reflector_rev = make(map[string]string)
	p.PodCoreAssignment = make(map[int]*PodCoreAssignment)
	p.SenderPods = make([]string, 0)
	p.ReceiverPods = make([]string, 0)
	p.PodPair = make(map[string]string)

	// get current kubeconfig
	var kubeconfig *string
	if home := homeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	// uses the current context in kubeconfig
	if config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig); err != nil {
		p.Log.Errorln("failed to build config from flags")
		panic(err.Error())
	} else {
		p.KubeConfig = config
	}

	// create the clientset
	if clientset, err := kubernetes.NewForConfig(p.KubeConfig); err != nil {
		p.Log.Errorln("failed to create the clientset")
		panic(err.Error())
	} else {
		p.ClientSet = clientset
	}

	go p.watch_pod_status()

	if p.Deps.Log == nil {
		p.Deps.Log = logging.ForPlugin(p.String())
	}
	p.PluginInitFinished = true

	nodes, err := p.ClientSet.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		p.Log.Errorln("failed to get nodes infomation")
		panic(err)
	}
	p.AssignedWorkerNumber = len(nodes.Items) - 1

	return nil
}

func (p *Plugin) String() string {
	return "kubernetes"
}

func (p *Plugin) Close() error {
	if err := p.ClientSet.AppsV1().Deployments(p.K8sNamespace).DeleteCollection(&metav1.DeleteOptions{}, metav1.ListOptions{}); err != nil {
		p.Log.Errorln(err)
		return err
	} else {
		p.Log.Infoln("successfully delete all deployment")
	}
	return nil
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" { // linux
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}

func (p *Plugin) Make_Topology(message rpctest.Message) error {
	for _, link := range message.Command.EmunetCreation.Emunet.Links {
		node1 := link.Node1.Name
		node2 := link.Node2.Name
		node1_inf := link.Node1Inf
		node2_inf := link.Node2Inf

		if pod, ok := p.PodSet[node1]; ok {
			if _, ok := pod[node1_inf]; ok {

			} else {
				pod[node1_inf] = ""
			}
		} else {
			p.PodSet[node1] = make(map[string]string)
			p.PodSet[node1][node1_inf] = ""
		}

		if pod, ok := p.PodSet[node2]; ok {
			if _, ok := pod[node2_inf]; ok {

			} else {
				pod[node2_inf] = ""
			}
		} else {
			p.PodSet[node2] = make(map[string]string)
			p.PodSet[node2][node2_inf] = ""
		}

		// generate topology-links
		p.MocknetTopology.Links = append(p.MocknetTopology.Links,
			Link{
				Name:    node1 + "-" + node2,
				Pod1:    node1,
				Pod2:    node2,
				Pod1inf: node1_inf,
				Pod2inf: node2_inf,
			},
		)
	}

	for podname, infset := range p.PodSet {
		infs := make([]string, 0)
		for inf := range infset {
			infs = append(infs, inf)
		}
		pod := Pod{
			Name: podname,
			Infs: infs,
		}
		p.MocknetTopology.Pods = append(p.MocknetTopology.Pods, pod)
	}

	if message.Command.EmunetCreation.Emunet.Type == "fat-tree" {
		host_numbers := 0
		for podname, _ := range p.PodSet {
			if strings.Contains(podname, "h") {
				host_numbers++
			}
		}
		k := 4
		for {
			if k*k*k/4 == host_numbers {
				break
			}
			k += 2
		}
		i := 0

		// strictly mark left side as sender and right side as receiver
		/*
			for _, pod := range p.MocknetTopology.Pods {
				if strings.Contains(pod.Name, "h") {
					switch_id_string := strings.Split(pod.Name, "s")[1]
					switch_id, err := strconv.Atoi(switch_id_string)
					if err != nil {
						panic(err)
					}
					if switch_id <= k*k*3/4 {
						// mark left side as sender
						p.SenderPods = append(p.SenderPods, pod.Name)
					} else {
						// mark right side as receiver
						p.ReceiverPods = append(p.ReceiverPods, pod.Name)
					}
				}
			}
		*/

		// randomly pair sender and receiver

		rand.Seed(time.Now().UnixMicro())
		rand.Shuffle(len(p.MocknetTopology.Pods), func(i, j int) {
			p.MocknetTopology.Pods[i], p.MocknetTopology.Pods[j] = p.MocknetTopology.Pods[j], p.MocknetTopology.Pods[i]
		})
		for _, pod := range p.MocknetTopology.Pods {
			if strings.Contains(pod.Name, "h") {
				if i%2 == 0 {
					p.SenderPods = append(p.SenderPods, pod.Name)
				} else {
					p.ReceiverPods = append(p.ReceiverPods, pod.Name)
				}
				i++
			}
		}

		p.Log.Println("p.SenderPods =", p.SenderPods)
		p.Log.Println("p.ReceiverPods =", p.ReceiverPods)
		for i = 0; i < len(p.SenderPods); i++ {
			p.PodPair[p.SenderPods[i]] = p.ReceiverPods[i]
			p.PodPair[p.ReceiverPods[i]] = p.SenderPods[i]
		}

		/*for _, pod := range message.Command.EmunetCreation.Emunet.Pods {
			if strings.Contains(pod.Name, "h") {
				p.SenderPods = append(p.SenderPods, pod.Name)
			}
		}*/
	}

	return nil
}

// cores: how many cores are assigned to a pod
func (p *Plugin) AffinityClusterPartition(message rpctest.Message) map[string]uint {
	workers := make(map[uint]string, 0)
	worker_assignment := affinity.AffinityClusterPartition(message, uint(p.AssignedWorkerNumber), 0.5, true) // -1 for master node
	p.Log.Infoln("the assignment is:", worker_assignment)

	for _, hostid := range worker_assignment {
		workers[hostid] = ""
	}
	p.AssignedWorkerNumber = len(workers)
	// p.Log.Infoln("len of workers is", len(workers))
	return worker_assignment
}

// switch_cores: how many cores are assigned to switch pod.
//
// switch_start_core: the assignment of switch pod start from which core.
//
// switch_start_core: the assignment of switch pod end till which core.
//
// host_cores: how many host pods are assigned to a core.
//
// host_start_core: the assignment of host pod start from which core.
//
// host_start_core: the assignment of host pod end till which core.
func (p *Plugin) Create_Deployment(
	assignment map[string]uint,
	switch_cores int,
	switch_start_core int,
	switch_end_core int,
	host_cores int,
	host_start_core int,
	host_end_core int,
) {
	assign_count := make(map[string]string)

	for i := 1; i <= p.AssignedWorkerNumber; i++ {
		var switch_end int
		var host_end int
		if host_cores == 1 {
			host_end = 0
		} else {
			host_end = host_cores - 1
		}
		if switch_cores == 1 {
			switch_end = 0
		} else {
			switch_end = switch_cores - 1
		}
		p.PodCoreAssignment[i] = &PodCoreAssignment{
			switchs: CoreAssignment{
				start: 0,
				end:   switch_end,
			},
			hosts: CoreAssignment{
				start: 0,
				end:   host_end,
			},
		}
	}

	for podname, hostid := range assignment {
		hostid_string := strconv.Itoa(int(hostid))
		if strings.Contains(podname, "h") {
			// assign host pod vpp to cores
			duration := host_start_core - host_end_core + 1
			start := p.PodCoreAssignment[int(hostid)].hosts.start
			end := p.PodCoreAssignment[int(hostid)].hosts.end
			if host_cores == 1 {
				assign_string := strconv.Itoa(start + host_start_core)
				p.PodCoreAssignment[int(hostid)].hosts.core_string = assign_string
				p.PodCoreAssignment[int(hostid)].hosts.start = (start + 1) % duration
				if _, ok := assign_count[hostid_string+"-"+"host"+"-"+assign_string]; ok {
					assign_count[hostid_string+"-"+"host"+"-"+assign_string] = assign_count[hostid_string+"-"+"host"+"-"+assign_string] + "," + podname
				} else {
					assign_count[hostid_string+"-"+"host"+"-"+assign_string] = podname
				}
			} else {
				if start < end {
					assign_string := strconv.Itoa(start+host_start_core) + "-" + strconv.Itoa(end+host_start_core)
					p.PodCoreAssignment[int(hostid)].hosts.core_string = assign_string
					if _, ok := assign_count[hostid_string+"-"+"host"+"-"+assign_string]; ok {
						assign_count[hostid_string+"-"+"host"+"-"+assign_string] = assign_count[hostid_string+"-"+"host"+"-"+assign_string] + "," + podname
					} else {
						assign_count[hostid_string+"-"+"host"+"-"+assign_string] = podname
					}
				} else {
					list1 := strconv.Itoa(host_start_core) + "-" + strconv.Itoa(start+host_start_core)
					list2 := strconv.Itoa(end+host_end_core) + "-" + strconv.Itoa(host_end_core)
					assign_string := list1 + "," + list2
					p.PodCoreAssignment[int(hostid)].hosts.core_string = assign_string
					if _, ok := assign_count[hostid_string+"-"+"host"+"-"+assign_string]; ok {
						assign_count[hostid_string+"-"+"host"+"-"+assign_string] = assign_count[hostid_string+"-"+"host"+"-"+assign_string] + "," + podname
					} else {
						assign_count[hostid_string+"-"+"host"+"-"+assign_string] = podname
					}
				}

				p.PodCoreAssignment[int(hostid)].hosts.start = (start + host_cores) % duration
				p.PodCoreAssignment[int(hostid)].hosts.end = (end + host_cores) % duration
			}

			deployment := make_deployment(podname, hostid, p.PodCoreAssignment[int(hostid)].hosts.core_string)
			if _, err := p.ClientSet.AppsV1().Deployments(p.K8sNamespace).Create(&deployment); err != nil {
				p.Log.Errorln("failed to create deployment:", podname)
				panic(err)
			} else {
				p.Log.Infoln("For host", podname, "in worker", hostid, ", core is", p.PodCoreAssignment[int(hostid)].hosts.core_string)
			}
			//p.Log.Infoln("For host", podname, "in worker", hostid, ", core is", p.PodCoreAssignment[int(hostid)].hosts.core_string)
		} else {
			// assign core to switch pod vpp
			duration := switch_start_core - switch_end_core + 1
			start := p.PodCoreAssignment[int(hostid)].switchs.start
			end := p.PodCoreAssignment[int(hostid)].switchs.end
			if switch_cores == 1 {
				assign_string := strconv.Itoa(start + switch_start_core)
				p.PodCoreAssignment[int(hostid)].switchs.core_string = assign_string
				p.PodCoreAssignment[int(hostid)].switchs.start = (start + 1) % duration
				if _, ok := assign_count[hostid_string+"-"+"switch"+"-"+assign_string]; ok {
					assign_count[hostid_string+"-"+"switch"+"-"+assign_string] = assign_count[hostid_string+"-"+"switch"+"-"+assign_string] + "," + podname
				} else {
					assign_count[hostid_string+"-"+"switch"+"-"+assign_string] = podname
				}
			} else {
				if start < end {
					assign_string := strconv.Itoa(start+switch_start_core) + "-" + strconv.Itoa(end+switch_start_core)
					p.PodCoreAssignment[int(hostid)].switchs.core_string = assign_string
					if _, ok := assign_count[hostid_string+"-"+"switch"+"-"+assign_string]; ok {
						assign_count[hostid_string+"-"+"switch"+"-"+assign_string] = assign_count[hostid_string+"-"+"switch"+"-"+assign_string] + "," + podname
					} else {
						assign_count[hostid_string+"-"+"switch"+"-"+assign_string] = podname
					}
				} else {
					list1 := strconv.Itoa(switch_start_core) + "-" + strconv.Itoa(start+switch_start_core)
					list2 := strconv.Itoa(end+switch_end_core) + "-" + strconv.Itoa(switch_end_core)
					assign_string := list1 + "," + list2
					p.PodCoreAssignment[int(hostid)].switchs.core_string = assign_string
					if _, ok := assign_count[hostid_string+"-"+"switch"+"-"+assign_string]; ok {
						assign_count[hostid_string+"-"+"switch"+"-"+assign_string] = assign_count[hostid_string+"-"+"switch"+"-"+assign_string] + "," + podname
					} else {
						assign_count[hostid_string+"-"+"switch"+"-"+assign_string] = podname
					}
				}

				p.PodCoreAssignment[int(hostid)].switchs.start = (start + switch_cores) % duration
				p.PodCoreAssignment[int(hostid)].switchs.end = (end + switch_cores) % duration
			}

			deployment := make_deployment(podname, hostid, p.PodCoreAssignment[int(hostid)].switchs.core_string)
			if _, err := p.ClientSet.AppsV1().Deployments(p.K8sNamespace).Create(&deployment); err != nil {
				p.Log.Errorln("failed to create deployment:", podname)
				panic(err)
			} else {
				p.Log.Infoln("For switch", podname, "in worker", hostid, ", core is", p.PodCoreAssignment[int(hostid)].switchs.core_string)
			}
			//p.Log.Infoln("For switch", podname, "in worker", hostid, ", core is", p.PodCoreAssignment[int(hostid)].switchs.core_string)
		}
	}
	for core, podname := range assign_count {
		p.Log.Infoln("core = ", core, "pod =", podname)
	}
}

func (p *Plugin) watch_pod_status() {
	for {
		p.PodList.Lock.Lock()
		temp_pods, err := p.ClientSet.CoreV1().Pods(p.K8sNamespace).List(metav1.ListOptions{})
		if err != nil {
			p.Log.Errorln(err)
			panic(err)
		}
		for _, pod := range temp_pods.Items {
			simplified_name := Parse_pod_name(pod.Name)
			if _, ok := p.PodList.List[simplified_name]; ok {
				p.PodList.List[simplified_name].Pod = pod
			} else {
				p.PodList.List[simplified_name] = &MocknetPod{
					Handled: false,
					Pod:     pod,
				}
			}

			// if pod restart, mark it with unhandled and reconfig
			if len(pod.Status.ContainerStatuses) >= 1 {
				if pod.Status.ContainerStatuses[0].RestartCount > p.PodList.List[simplified_name].RestartCount {
					p.Log.Warningln("detectd a restart of pod", simplified_name)
					p.PodList.List[simplified_name].RestartCount = pod.Status.ContainerStatuses[0].RestartCount
					p.PodList.List[simplified_name].Handled = false
				}
			}

			if _, ok := p.Pod_name_reflector[pod.Name]; !ok {
				p.Pod_name_reflector[pod.Name] = simplified_name
				p.Pod_name_reflector_rev[simplified_name] = pod.Name
			}
		}
		delete(p.PodList.List, "")
		//p.Log.Infoln("the length of podlist is", len(p.PodList.List))
		p.PodList.Lock.Unlock()
		time.Sleep(POD_STATUS_WATCH_INTERVAL)
	}
}

func (p *Plugin) Pod_Tap_Config(pod coreV1.Pod) error {
	var stderr bytes.Buffer
	simplified_name := Parse_pod_name(pod.Name)
	p.Log.Infoln("configuring tap interface for pod", simplified_name)
	cp := strings.Split(pod.Status.PodIP, ".") // conrol plane ip
	data_plane_ip := "10.1." + cp[2] + "." + cp[3] + "/16"
	cmd :=
		`ip route add 10.1.0.0/16 dev tap0
ip addr add dev tap0 `

	create_cmd := exec.Command("kubectl", "exec", pod.Name, "--", "/bin/bash", "-c", cmd+data_plane_ip)
	create_cmd.Stderr = &stderr
	err := create_cmd.Run()
	if err != nil {
		if !strings.Contains(stderr.String(), "File exists") {
			p.Log.Warningln(err.Error(), stderr.String(), ", for pod", simplified_name)
			return err
		}
	}
	p.Log.Infoln("config tap interface for pod", pod.Name, "finished")

	return nil
}

func (p *Plugin) Is_Creation_Completed(pod_name string) bool {
	p.PodList.Lock.Lock()
	defer p.PodList.Lock.Unlock()
	if mocknet_pod, ok := p.PodList.List[pod_name]; !ok {
		//p.Log.Infoln("pod", pod_name, "don't exist in podlist")
		return false
	} else {
		//p.Log.Infoln(mocknet_pod.Pod.Status.Conditions)
		flag := false
		for _, condition := range mocknet_pod.Pod.Status.Conditions {
			if condition.Type == coreV1.PodConditionType("Ready") && condition.Status == coreV1.ConditionTrue {
				flag = true
			}
		}
		if !flag {
			return false
		}
		split_ip := strings.Split(mocknet_pod.Pod.Status.PodIP, ".")
		//p.Log.Infoln("split_ip =", split_ip)
		if len(split_ip) != 4 {
			return false
		}
	}
	return true
}

func make_deployment(name string, worker_id uint, core_id string) appsv1.Deployment {
	var replica int32 = 1
	privileged := true
	host := "worker" + strconv.Itoa(int(worker_id))

	//sed -i "44c main-core ${CORE_ASSGIENMENT}" /etc/vpp/startup.conf
	//sed -i "47c corelist-workers ${WORKER_CORE_ASSGIENMENT}" /etc/vpp/startup.conf
	cmd :=
		`sed -i "47c corelist-workers ${CORE_ASSGIENMENT}" /etc/vpp/startup.conf
mkdir /run/vpp
vpp -c /etc/vpp/startup.conf &

while [ ! -e "/run/vpp/api.sock" ]
do 
	sleep 1
	echo "api.sock hasn't been created, waitting"
done 

while true
do 
sleep 60
done 
`

	return appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "mocknet-" + name,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replica,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "mocknet",
				},
			},
			Template: coreV1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "mocknet",
					},
					Annotations: map[string]string{
						"contivpp.io/microservice-label": "mocknet-pod-" + name,
					},
				},
				Spec: coreV1.PodSpec{
					NodeSelector: map[string]string{
						"kubernetes.io/hostname": host,
					},
					Containers: []coreV1.Container{
						{
							Name:            "mocknet-pod",
							Image:           "pengyang2157/mocknet-pod:v1.5",
							ImagePullPolicy: coreV1.PullPolicy("IfNotPresent"),
							SecurityContext: &coreV1.SecurityContext{
								Privileged: &privileged,
							},
							ReadinessProbe: &coreV1.Probe{
								Handler: coreV1.Handler{
									Exec: &coreV1.ExecAction{
										Command: []string{
											"/home/probe",
										},
									},
								},
								PeriodSeconds:       1,
								InitialDelaySeconds: 5,
							},
							Command: []string{
								"bash", "-c", cmd,
							},
							Env: []coreV1.EnvVar{
								{
									Name:  "MICROSERVICE_LABEL",
									Value: "mocknet-pod-" + name,
								},
								{
									Name: "HOST_IP",
									ValueFrom: &coreV1.EnvVarSource{
										FieldRef: &coreV1.ObjectFieldSelector{
											FieldPath: "status.hostIP",
										},
									},
								},
								{
									Name:  "CORE_ASSGIENMENT",
									Value: core_id,
								},
							},
							VolumeMounts: []coreV1.VolumeMount{
								{
									Name:      "host-memif-path",
									MountPath: "/run/vpp/",
								},
							},
						},
					},
					Volumes: []coreV1.Volume{
						{
							Name: "host-memif-path",
							VolumeSource: coreV1.VolumeSource{
								HostPath: &coreV1.HostPathVolumeSource{
									Path: "/var/run/mocknet/" + name,
								},
							},
						},
					},
				},
			},
		},
	}
}

func (p *Plugin) Assign_VTEP() map[string]string {
	node_infos := make(map[string]string, 0)
	Nodes, err := p.ClientSet.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		p.Log.Errorln(err)
		panic(err)
	}

	// 10.2.0.X/24 as vtep ip address, namely 40Gbps interface
	vtep_count := 1
	for _, node := range Nodes.Items {
		name := node.Name
		var node_ip string
		for _, addr := range node.Status.Addresses {
			if addr.Type == coreV1.NodeAddressType(coreV1.NodeInternalIP) {
				node_ip = addr.Address
				break
			}
		}

		vtep_ip := VTEP_PREFIX + strconv.Itoa(vtep_count)
		key := "/mocknet/nodeinfo/" + name
		value := "name:" + name + "," + "nodeip:" + node_ip + "," + "vtepip:" + vtep_ip
		node_infos[key] = value

		nodeinfo := Nodeinfo{
			Name:   name,
			Nodeip: node_ip,
			Vtepip: vtep_ip,
		}
		p.Nodeinfos[node_ip] = nodeinfo

		vtep_count += 1
	}
	return node_infos
}

func Parse_pod_name(logic_name string) string {
	split_name := strings.Split(logic_name, "-")
	return split_name[1]
}
