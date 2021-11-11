package kubernetes

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"mocknet/plugins/server/rpctest"

	appsv1 "k8s.io/api/apps/v1"
	coreV1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"go.ligato.io/cn-infra/v2/logging"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	k8s_rest "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
)

const (
	VTEP_PREFIX  = "10.2.0."
	VTEP_POSTFIX = "/24"
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
	Nodeinfos            map[string]Nodeinfo
	AssignedWorkerNumber int
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

	if p.Deps.Log == nil {
		p.Deps.Log = logging.ForPlugin(p.String())
	}

	return nil
}

func (p *Plugin) String() string {
	return "kubernetes"
}

func (p *Plugin) Close() error {
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

	}

	return nil
}

func (p *Plugin) Create_Deployment(message rpctest.Message) []string {
	pod_names := make([]string, 0)
	workers := make(map[string]string)
	num := 0
	for _, pod := range message.Command.EmunetCreation.Emunet.Pods {
		deployment := make_deployment(pod.Name, num) // 获取节点的个数并创建相应deployment数据

		if _, err := p.ClientSet.AppsV1().Deployments(p.K8sNamespace).Create(&deployment); err != nil {
			p.Log.Errorln("failed to create deployment:", pod.Name)
			panic(err)
		} else {
			p.Log.Infoln("successfully created deployment", pod.Name)
		}
		num++
	}

	// wait for creation finished
	p.Log.Infoln("waiting for pods creation finish")
	for {
		temp_pods, err := p.ClientSet.CoreV1().Pods(p.K8sNamespace).List(metav1.ListOptions{})
		if err != nil {
			p.Log.Errorln(err)
			panic(err)
		}
		if is_creation_completed(temp_pods.Items, message.Command.EmunetCreation.Emunet.Pods) {
			for _, pod := range temp_pods.Items {
				pod_names = append(pod_names, pod.Name)
				workers[pod.Status.HostIP] = ""
			}
			p.Log.Infoln("pods creation finished!")
			break
		}
	}
	p.AssignedWorkerNumber = len(workers)
	p.Log.Infoln("AssignedWorkerNumber = ", p.AssignedWorkerNumber)
	return pod_names
}

func (p *Plugin) Pod_Tap_Config() error {
	// create tap interface in pod-side vpp and write route table to pod linux namespace
	// flannel CNI default pod cidr is 10.0.0.0/16(control plane), use 10.1.0.0/16 as data plane cidr
	pods, _ := p.ClientSet.CoreV1().Pods(p.K8sNamespace).List(metav1.ListOptions{})
	for _, pod := range pods.Items {
		p.Log.Infoln("configing tap interface for pod ", pod.Name)
		cp := strings.Split(pod.Status.PodIP, ".") // conrol plane ip
		data_plane_ip := "10.1." + cp[2] + "." + cp[3] + "/16"
		cmd :=
			`OUTPUT="$(ip addr | grep "tap0")"
		while [ -z "$OUTPUT" ]
		do 
			echo ""
		done
		
		ip route add 10.1.0.0/16 dev tap0
		ip addr add dev tap0 `

		req := p.ClientSet.CoreV1().RESTClient().Post().
			Resource("pods").
			Name(pod.Name).
			Namespace(p.K8sNamespace).
			SubResource("exec").
			VersionedParams(&coreV1.PodExecOptions{
				Command: []string{"sh", "-c", cmd + data_plane_ip},
				Stdin:   true,
				Stdout:  true,
				Stderr:  true,
				TTY:     false,
			}, scheme.ParameterCodec)

		executor, err := remotecommand.NewSPDYExecutor(p.KubeConfig, "POST", req.URL())
		if err != nil {
			panic(err)
		}
		var stdout, stderr bytes.Buffer
		if err = executor.Stream(remotecommand.StreamOptions{
			Stdin:  strings.NewReader(""),
			Stdout: &stdout,
			Stderr: &stderr,
		}); err != nil {
			p.Log.Infoln(err)
		}
		// 返回数据
		ret := map[string]string{"stdout": stdout.String(), "stderr": stderr.String(), "pod_name": "mocknet-h1-fc4c5df56-tdk6f"}
		p.Log.Infoln(ret)
	}
	p.Log.Infoln("pods tap config finished")

	return nil
}

func is_creation_completed(podlist []coreV1.Pod, pods []*rpctest.Pod) bool {
	if len(podlist) != len(pods) {
		return false
	}
	for _, pod := range podlist {
		if pod.Status.Phase != "Running" {
			return false
		}
		if len(pod.Status.HostIP) <= 4 || len(pod.Status.PodIP) <= 4 {
			return false
		}
	}
	return true
}

func make_configmap() coreV1.ConfigMap {
	EtcdConfData :=
		`insecure-transport: true
dial-timeout: 10000000000
allow-delayed-start: true
endpoints:`
	return coreV1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "etcd-cfg",
			Namespace: "default",
			Labels: map[string]string{
				"name": "etcd-cfg",
			},
		},
		Data: map[string]string{
			"etcd.conf": EtcdConfData,
		},
	}
}

func make_deployment(name string, num int) appsv1.Deployment {
	var replica int32 = 1
	privileged := true
	var role string
	if num%2 == 0 {
		role = "worker1"
	} else {
		role = "worker2"
	}
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
						"mocknetrole": role,
					},
					Containers: []coreV1.Container{
						{
							Name:  "vpp-agent",
							Image: "ligato/vpp-agent:latest",
							SecurityContext: &coreV1.SecurityContext{
								Privileged: &privileged,
							},
							Env: []coreV1.EnvVar{
								{
									Name:  "ETCD_CONFIG",
									Value: "/etc/etcd/etcd.conf",
								},
								{
									Name:  "MICROSERVICE_LABEL",
									Value: "mocknet-pod-" + name,
								},
								{
									Name: "MY_HOST_IP",
									ValueFrom: &coreV1.EnvVarSource{
										FieldRef: &coreV1.ObjectFieldSelector{
											FieldPath: "status.hostIP",
										},
									},
								},
							},
							VolumeMounts: []coreV1.VolumeMount{
								{
									Name:      "etcd-cfg",
									MountPath: "/etc/etcd",
								},
								{
									Name:      "host-memif-path",
									MountPath: "/run/vpp/",
								},
							},
						},
					},
					Volumes: []coreV1.Volume{
						{
							Name: "etcd-cfg",
							VolumeSource: coreV1.VolumeSource{
								HostPath: &coreV1.HostPathVolumeSource{
									Path: "/opt/etcd",
								},
							},
						},
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

	// 10.2.0.X/24 as vtep ip address
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
