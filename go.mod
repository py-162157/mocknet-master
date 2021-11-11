module mocknet

go 1.16

require (
	github.com/fluent/fluent-logger-golang v1.4.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/googleapis/gnostic v0.2.0 // indirect
	github.com/gorilla/mux v1.7.1 // indirect
	github.com/imdario/mergo v0.3.7 // indirect
	github.com/json-iterator/go v1.1.11 // indirect
	github.com/ligato/cn-infra v2.2.0+incompatible // indirect
	github.com/onsi/gomega v1.10.3 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/sirupsen/logrus v1.6.0 // indirect
	github.com/stretchr/testify v1.7.0 // indirect
	github.com/tinylib/msgp v1.1.0 // indirect
	github.com/unrolled/render v1.0.1-0.20190325150441-1ac792296fd4 // indirect
	go.etcd.io/etcd/client/v3 v3.5.0
	go.ligato.io/cn-infra v2.2.0+incompatible
	go.ligato.io/cn-infra/v2 v2.5.0-alpha.0.20200313154441-b0d4c1b11c73
	golang.org/x/net v0.0.0-20210825183410-e898025ed96a
	golang.org/x/sys v0.0.0-20210603081109-ebe580a85c40 // indirect
	golang.org/x/text v0.3.7 // indirect
	golang.org/x/time v0.0.0-20190921001708-c4c64cad1fd0 // indirect
	google.golang.org/genproto v0.0.0-20210602131652-f16073e35f0c // indirect
	google.golang.org/grpc v1.40.0
	google.golang.org/protobuf v1.27.1
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
	k8s.io/api v0.17.1
	k8s.io/apimachinery v0.17.1
	k8s.io/client-go v11.0.0+incompatible
	sigs.k8s.io/yaml v1.2.0 // indirect

)

replace (
	k8s.io/api => k8s.io/api v0.17.1
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.17.1
	k8s.io/apimachinery => k8s.io/apimachinery v0.17.1
	k8s.io/apiserver => k8s.io/apiserver v0.17.1
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.17.1
	k8s.io/client-go => k8s.io/client-go v0.17.1
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.17.1
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.17.1
	k8s.io/code-generator => k8s.io/code-generator v0.17.1
	k8s.io/component-base => k8s.io/component-base v0.17.1
	k8s.io/cri-api => k8s.io/cri-api v0.17.1
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.17.1
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.17.1
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.17.1
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.17.1
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.17.1
	k8s.io/kubectl => k8s.io/kubectl v0.17.1
	k8s.io/kubelet => k8s.io/kubelet v0.17.1
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.17.1
	k8s.io/metrics => k8s.io/metrics v0.17.1
	k8s.io/node-api => k8s.io/node-api v0.17.1
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.17.1
	k8s.io/sample-cli-plugin => k8s.io/sample-cli-plugin v0.17.1
	k8s.io/sample-controller => k8s.io/sample-controller v0.17.1
)
