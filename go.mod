module github.com/kubernetes-sigs/cri-tools

go 1.15

require (
	github.com/docker/distribution v2.8.1+incompatible
	github.com/docker/docker v20.10.13+incompatible
	github.com/docker/go-units v0.4.0
	github.com/golang/glog v1.0.0
	github.com/golang/protobuf v1.5.2
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.18.1
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/runc v1.1.0
	github.com/opencontainers/selinux v1.10.0
	github.com/pborman/uuid v1.2.1
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.8.1
	github.com/urfave/cli/v2 v2.5.0
	golang.org/x/net v0.0.0-20211209124913-491a49abca63
	golang.org/x/sys v0.0.0-20220114195835-da31bd327af9
	google.golang.org/grpc v1.45.0 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
	k8s.io/api v0.23.1
	k8s.io/apimachinery v0.23.1
	k8s.io/client-go v1.5.2
	k8s.io/cri-api v0.23.1
	k8s.io/kubectl v0.23.1
	k8s.io/kubernetes v1.23.1
	sigs.k8s.io/yaml v1.3.0
)

replace (
	k8s.io/api => k8s.io/kubernetes/staging/src/k8s.io/api v0.0.0-20220215073202-d899c39ca302
	k8s.io/apiextensions-apiserver => k8s.io/kubernetes/staging/src/k8s.io/apiextensions-apiserver v0.0.0-20220215073202-d899c39ca302
	k8s.io/apimachinery => k8s.io/kubernetes/staging/src/k8s.io/apimachinery v0.0.0-20220215073202-d899c39ca302
	k8s.io/apiserver => k8s.io/kubernetes/staging/src/k8s.io/apiserver v0.0.0-20220215073202-d899c39ca302
	k8s.io/cli-runtime => k8s.io/kubernetes/staging/src/k8s.io/cli-runtime v0.0.0-20220215073202-d899c39ca302
	k8s.io/client-go => k8s.io/kubernetes/staging/src/k8s.io/client-go v0.0.0-20220215073202-d899c39ca302
	k8s.io/cloud-provider => k8s.io/kubernetes/staging/src/k8s.io/cloud-provider v0.0.0-20220215073202-d899c39ca302
	k8s.io/cluster-bootstrap => k8s.io/kubernetes/staging/src/k8s.io/cluster-bootstrap v0.0.0-20220215073202-d899c39ca302
	k8s.io/code-generator => k8s.io/kubernetes/staging/src/k8s.io/code-generator v0.0.0-20220215073202-d899c39ca302
	k8s.io/component-base => k8s.io/kubernetes/staging/src/k8s.io/component-base v0.0.0-20220215073202-d899c39ca302
	k8s.io/component-helpers => k8s.io/kubernetes/staging/src/k8s.io/component-helpers v0.0.0-20220215073202-d899c39ca302
	k8s.io/controller-manager => k8s.io/kubernetes/staging/src/k8s.io/controller-manager v0.0.0-20220215073202-d899c39ca302
	k8s.io/cri-api => k8s.io/kubernetes/staging/src/k8s.io/cri-api v0.0.0-20220215073202-d899c39ca302
	k8s.io/csi-translation-lib => k8s.io/kubernetes/staging/src/k8s.io/csi-translation-lib v0.0.0-20220215073202-d899c39ca302
	k8s.io/kube-aggregator => k8s.io/kubernetes/staging/src/k8s.io/kube-aggregator v0.0.0-20220215073202-d899c39ca302
	k8s.io/kube-controller-manager => k8s.io/kubernetes/staging/src/k8s.io/kube-controller-manager v0.0.0-20220215073202-d899c39ca302
	k8s.io/kube-proxy => k8s.io/kubernetes/staging/src/k8s.io/kube-proxy v0.0.0-20220215073202-d899c39ca302
	k8s.io/kube-scheduler => k8s.io/kubernetes/staging/src/k8s.io/kube-scheduler v0.0.0-20220215073202-d899c39ca302
	k8s.io/kubectl => k8s.io/kubernetes/staging/src/k8s.io/kubectl v0.0.0-20220215073202-d899c39ca302
	k8s.io/kubelet => k8s.io/kubernetes/staging/src/k8s.io/kubelet v0.0.0-20220215073202-d899c39ca302
	k8s.io/kubernetes => k8s.io/kubernetes v1.24.0-alpha.2.0.20220215073202-d899c39ca302
	k8s.io/legacy-cloud-providers => k8s.io/kubernetes/staging/src/k8s.io/legacy-cloud-providers v0.0.0-20220215073202-d899c39ca302
	k8s.io/metrics => k8s.io/kubernetes/staging/src/k8s.io/metrics v0.0.0-20220215073202-d899c39ca302
	k8s.io/mount-utils => k8s.io/kubernetes/staging/src/k8s.io/mount-utils v0.0.0-20220215073202-d899c39ca302
	k8s.io/pod-security-admission => k8s.io/kubernetes/staging/src/k8s.io/pod-security-admission v0.0.0-20220215073202-d899c39ca302
	k8s.io/sample-apiserver => k8s.io/kubernetes/staging/src/k8s.io/sample-apiserver v0.0.0-20220215073202-d899c39ca302
)
