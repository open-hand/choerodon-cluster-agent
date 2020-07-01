package apis

import (
	"github.com/choerodon/choerodon-cluster-agent/pkg/apis/schemafunc"
	"k8s.io/apimachinery/pkg/runtime"
)

func init() {
	schemafunc.AddSchemaFuncs = append(schemafunc.AddSchemaFuncs, AddToScheme)
}

// AddToSchemes may be used to add all resources defined in the project to a Scheme
var AddToSchemes runtime.SchemeBuilder

// AddToScheme adds all Resources to the Scheme
func AddToScheme(s *runtime.Scheme) error {
	return AddToSchemes.AddToScheme(s)
}
