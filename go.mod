module github.com/choerodon/choerodon-cluster-agent

go 1.13

require (
	github.com/choerodon/helm v0.0.0-20200317082929-220d898586e8
	github.com/ghodss/yaml v1.0.1-0.20190212211648-25d852aebe32
	github.com/gin-gonic/gin v1.5.0
	github.com/go-openapi/spec v0.19.7
	github.com/gobuffalo/packr/v2 v2.8.0
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/golang/protobuf v1.3.2
	github.com/gorilla/websocket v1.4.1
	github.com/hashicorp/go-cleanhttp v0.5.1
	github.com/maorfr/helm-plugin-utils v0.0.0
	github.com/mitchellh/go-homedir v1.1.0
	github.com/operator-framework/operator-sdk v0.15.2
	github.com/pkg/errors v0.9.1
	github.com/qri-io/jsonschema v0.1.1
	github.com/sirupsen/logrus v1.4.2
	github.com/spf13/cobra v0.0.6
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.5.1
	github.com/vinkdong/gox v0.0.0-20200309024842-3e4422374a31
	gopkg.in/yaml.v2 v2.2.8
	k8s.io/api v0.17.3
	k8s.io/apiextensions-apiserver v0.17.3
	k8s.io/apimachinery v0.17.4
	k8s.io/apimachinery/pkg/apimachinery v0.0.0-00010101000000-000000000000
	k8s.io/cli-runtime v0.17.3
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/code-generator v0.17.2
	k8s.io/gengo v0.0.0-20200205140755-e0e292d8aa12
	k8s.io/helm v2.16.3+incompatible
	k8s.io/kube-openapi v0.0.0-20200204173128-addea2498afe
	k8s.io/kubectl v0.17.3
	k8s.io/kubernetes v1.17.3
	k8s.io/metrics v0.17.2
	sigs.k8s.io/controller-runtime v0.5.1
)

replace (
	k8s.io/api => k8s.io/api v0.17.2
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.17.2
	k8s.io/apimachinery => k8s.io/apimachinery v0.17.4-beta.0
	k8s.io/apiserver => k8s.io/apiserver v0.17.2
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.17.2
	k8s.io/client-go => k8s.io/client-go v0.17.2
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.17.2
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.17.2
	k8s.io/code-generator => k8s.io/code-generator v0.17.4-beta.0
	k8s.io/component-base => k8s.io/component-base v0.17.2
	k8s.io/cri-api => k8s.io/cri-api v0.17.4-beta.0
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.17.2
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.17.2
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.17.2
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.17.2
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.17.2
	k8s.io/kubectl => k8s.io/kubectl v0.17.2
	k8s.io/kubelet => k8s.io/kubelet v0.17.2
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.17.2
	k8s.io/metrics => k8s.io/metrics v0.17.2
	k8s.io/node-api => k8s.io/node-api v0.17.2
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.17.2
	k8s.io/sample-cli-plugin => k8s.io/sample-cli-plugin v0.17.2
	k8s.io/sample-controller => k8s.io/sample-controller v0.17.2
)

replace (
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v13.3.2+incompatible
	github.com/docker/distribution => github.com/docker/distribution v0.0.0-20191216044856-a8371794149d
	github.com/docker/docker => github.com/docker/docker v1.4.2-0.20200203170920-46ec8731fbce
	github.com/maorfr/helm-plugin-utils => github.com/maorfr/helm-plugin-utils v0.0.0-20200216074820-36d2fcf6ae86
	k8s.io/apimachinery/pkg/apimachinery => ./certificate/apimachinery
)
