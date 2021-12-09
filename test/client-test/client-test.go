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
	/*req := clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name("mocknet-h1s2-7d7ddd7b97-kcrh5").
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
	deployment := make_deployment(1)
	_, err = clientset.AppsV1().Deployments(namespace).Create(&deployment)
	if err != nil {
		panic(err)
	} else {
		fmt.Println("successfully create deployment")
	}

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
					Annotations: map[string]string{
						"contivpp.io/custom-if":          "memif1/memif/stub, memif2/memif/stub, memif3/memif/stub, memif4/memif/stub, memif5/memif/stub",
						"contivpp.io/microservice-label": "mocknet",
					},
				},
				Spec: coreV1.PodSpec{
					NodeSelector: map[string]string{
						"kubernetes.io/hostname": "worker2",
					},
					RestartPolicy: coreV1.RestartPolicy("Always"),
					Containers: []coreV1.Container{
						{
							Name:  "vpp-agent",
							Image: "ligato/vpp-agent:latest",
							SecurityContext: &coreV1.SecurityContext{
								Privileged: &privilege,
							},
							ImagePullPolicy: coreV1.PullPolicy("IfNotPresent"),
							Env: []coreV1.EnvVar{
								{
									Name:  "ETCD_CONFIG",
									Value: "/etc/etcd/etcd.conf",
								},
								{
									Name:  "MICROSERVICE_LABEL",
									Value: "mocknet",
								},
							},
							VolumeMounts: []coreV1.VolumeMount{
								{
									Name:      "etcvpp",
									MountPath: "/etc/vpp",
								},
							},
						},
					},
					Volumes: []coreV1.Volume{
						{
							Name: "etcvpp",
							VolumeSource: coreV1.VolumeSource{
								HostPath: &coreV1.HostPathVolumeSource{
									Path: "/etc/vpp",
								},
							},
						},
					},
				},
			},
		},
	}
}
