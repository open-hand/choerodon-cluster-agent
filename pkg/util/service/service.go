package service

import (
	"fmt"
	"github.com/choerodon/choerodon-cluster-agent/pkg/model"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"strings"
)

const (
	NetworkServiceInstancesAnno = "choerodon.io/network-service-instances"
	NetworkServiceInstancesSep  = "+"
)

func IsInstancedService(service *v1.Service) bool {
	if service.Spec.Selector != nil {
		return false
	}
	if code, ok := service.Annotations[NetworkServiceInstancesAnno]; !ok || code == "" {
		return false
	}
	return true
}

func InstancedPodSelector(service *v1.Service) (labels.Selector, error) {
	instanceCodes := strings.Split(service.Annotations[NetworkServiceInstancesAnno], NetworkServiceInstancesSep)
	var selector labels.Selector
	var err error
	if len(instanceCodes) == 1 {
		selector = labels.Set(map[string]string{model.ReleaseLabel: instanceCodes[0]}).AsSelectorPreValidated()
	} else {
		ps := &metav1.LabelSelector{
			MatchExpressions: []metav1.LabelSelectorRequirement{
				{model.ReleaseLabel, metav1.LabelSelectorOpIn, instanceCodes},
			},
		}
		selector, err = metav1.LabelSelectorAsSelector(ps)
		if err != nil {
			return nil, fmt.Errorf("%q has invalid label selector: %v", instanceCodes, err)
		}
	}
	return selector, nil
}
