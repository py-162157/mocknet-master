package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	//"github.com/coreos/etcd/etcdctl/ctlv2/command"
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

	etcd := make_etcd()
	_, err = clientset.AppsV1().StatefulSets(namespace).Create(&etcd)
	if err != nil {
		fmt.Println("failed to create StatefulSet!")
		fmt.Println(err)
	} else {
		fmt.Println("successfully to created StatefulSet!")
	}

	service := make_service()
	_, err = clientset.CoreV1().Services(namespace).Create(&service)
	if err != nil {
		fmt.Println("failed to create Service!")
		fmt.Println(err)
	} else {
		fmt.Println("successfully to created Service!")
	}
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" { // linux
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}

func make_etcd() appsv1.StatefulSet {
	etcd_command :=
		`echo "$HOST_IP" | grep -q ':'
if [ "$?" -eq "0" ];
then
	HOST_IP="[$HOST_IP]"
fi
etcdctl get --endpoints=$HOST_IP:32379 /`
	return appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StatefulSet",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mocknet-etcd",
			Namespace: "default",
			Labels: map[string]string{
				"app": "mocknet-etcd",
			},
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: "mocknet-etcd",
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "mocknet-etcd",
				},
			},
			Template: coreV1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "mocknet-etcd",
					},
					Annotations: map[string]string{
						"scheduler.alpha.kubernetes.io/critical-pod": "''",
					},
				},
				Spec: coreV1.PodSpec{
					Tolerations: []coreV1.Toleration{
						{
							Operator: "Exists",
						},
						{
							Key:      "CriticalAddonsOnly",
							Operator: "Exists",
						},
					},
					NodeSelector: map[string]string{
						"mocknetrole": "master",
					},
					HostNetwork: true,
					Containers: []coreV1.Container{
						{
							Name:            "mocknet-etcd",
							Image:           "quay.io/coreos/etcd:latest",
							ImagePullPolicy: coreV1.PullIfNotPresent,
							Env: []coreV1.EnvVar{
								{
									Name: "MOCKNET_ETCD_IP",
									ValueFrom: &coreV1.EnvVarSource{
										FieldRef: &coreV1.ObjectFieldSelector{
											FieldPath: "status.podIP",
										},
									},
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
									Name:  "ETCDCTL_API",
									Value: "3",
								},
							},
							Command: []string{
								"/bin/sh",
							},
							Args: []string{
								"-c", "/usr/local/bin/etcd --name=mocknet-etcd --data-dir=/var/etcd/mocknet-data --advertise-client-urls=http://0.0.0.0:12379 --listen-client-urls=http://0.0.0.0:12379 --listen-peer-urls=http://0.0.0.0:12380",
							},
							VolumeMounts: []coreV1.VolumeMount{
								{
									Name:      "var-etcd",
									MountPath: "/var/etcd/",
								},
							},
							LivenessProbe: &coreV1.Probe{
								Handler: coreV1.Handler{
									Exec: &coreV1.ExecAction{
										Command: []string{
											"/bin/sh", "-c", etcd_command,
										},
									},
								},
								PeriodSeconds:       3,
								InitialDelaySeconds: 20,
							},
							/*Resources: coreV1.ResourceRequirements{
								Requests: coreV1.ResourceList{
									"cpu": *resource.NewQuantity(80000000, "DecimalSI"),
								},
							},*/
						},
					},
					Volumes: []coreV1.Volume{
						{
							Name: "var-etcd",
							VolumeSource: coreV1.VolumeSource{
								HostPath: &coreV1.HostPathVolumeSource{
									Path: "/var/etcd",
								},
							},
						},
					},
				},
			},
		},
	}
}

func make_service() coreV1.Service {
	return coreV1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mocknet-etcd",
			Namespace: "default",
		},
		Spec: coreV1.ServiceSpec{
			Type: coreV1.ServiceType("NodePort"),
			Selector: map[string]string{
				"app": "mocknet-etcd",
			},
			Ports: []coreV1.ServicePort{
				{
					Port:     12379,
					NodePort: 32379,
				},
			},
		},
	}
}
