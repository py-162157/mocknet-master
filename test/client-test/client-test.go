package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	appsv1 "k8s.io/api/apps/v1"
	coreV1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	var kubeconfig *string
	if home := homeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()
	// uses the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	namespace := "default"
	deployment := make_deployment(1)
	if _, err := clientset.AppsV1().Deployments(namespace).Create(&deployment); err != nil {
		fmt.Println("failed to create deployment:")
		panic(err)
	} else {
		fmt.Println("successfully created deployment")
	}
	/*pods, err := clientset.CoreV1().Pods(namespace).List(metav1.ListOptions{})
	for _, pod := range pods.Items {
		fmt.Println(pod.Status.ContainerStatuses[0].ContainerID)
	}*/
	/*req := clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name("mocknet-h2-6575ff7f7c-rrwf7").
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(&coreV1.PodExecOptions{
			Command: []string{"vppctl", "-s", ":5002", "create", "tap"},
			Stdin:   true,
			Stdout:  true,
			Stderr:  true,
			TTY:     false,
		}, scheme.ParameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		panic(err)
	}
	var stdout, stderr bytes.Buffer
	if err = executor.Stream(remotecommand.StreamOptions{
		Stdin:  strings.NewReader(""),
		Stdout: &stdout,
		Stderr: &stderr,
	}); err != nil {
		fmt.Println(err)
	}*/
	/*deployment := make_deployment(1)
	_, err = clientset.AppsV1().Deployments(namespace).Create(&deployment)
	if err != nil {
		panic(err)
	} else {
		fmt.Println("successfully create deployment")
	}*/

}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" { // linux
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}

func make_configmap() coreV1.ConfigMap {
	EtcdConfData :=
		`insecure-transport: true
dial-timeout: 10000000000
allow-delayed-start: true
endpoints:
  - "0.0.0.0:32379"` //格式很重要，务必按照此来，否则vpp-agent会解析失败
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

func make_deployment(replica int32) appsv1.Deployment {
	cmd :=
		`
vpp -c /etc/vpp/startup.conf &

while [ ! -e "/run/vpp/api.sock" ]
do 
	sleep 1
done 

vppctl -s :5002 create tap
vppctl -s :5002 set int state tap0 up

while true
do 
	sleep 60
done 
`
	privilege := true
	return appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "mocknet-deployment",
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
				},
				Spec: coreV1.PodSpec{
					NodeSelector: map[string]string{
						"kubernetes.io/hostname": "worker1",
					},
					RestartPolicy: coreV1.RestartPolicy("Always"),
					Containers: []coreV1.Container{
						{
							Name:  "vpp-agent",
							Image: "pengyang2157/mocknet-pod:v1.0",
							SecurityContext: &coreV1.SecurityContext{
								Privileged: &privilege,
							},
							ImagePullPolicy: coreV1.PullPolicy("IfNotPresent"),
							Command: []string{
								"bash", "-c", cmd,
							},
						},
					},
				},
			},
		},
	}
}
