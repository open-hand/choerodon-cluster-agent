package service

import (
	"context"
	serviceutil "github.com/choerodon/choerodon-cluster-agent/pkg/util/service"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *ReconcileServicePod) deleteInstancedPod(pod *v1.Pod) error {
	return r.updatePodServiceMemberships(pod)
}

func (r *ReconcileServicePod) checkInstancedPod(pod *v1.Pod) error {
	return r.updatePodServiceMemberships(pod)
}

func (r *ReconcileServicePod) updatePodServiceMemberships(pod *v1.Pod) error {
	services, err := r.GetPodServices(pod)
	if err != nil {
		// don't log this error because this function makes pointless
		// errors when no services match.
		return err
	}
	sr := ReconcileService{
		client: r.client,
	}
	for _, s := range services {
		if err := sr.checkInstancedService(&s); err != nil {
			return err
		}
	}
	return nil
}

func (r *ReconcileServicePod) GetPodServices(pod *v1.Pod) ([]v1.Service, error) {

	serviceList := make([]v1.Service, 0)
	services := &v1.ServiceList{}

	opts := &client.ListOptions{
		Namespace: pod.Namespace,
	}
	if err := r.client.List(context.TODO(), services, opts); err != nil {
		return nil, err
	}

	for _, s := range services.Items {

		if !serviceutil.IsInstancedService(&s) {
			continue
		}

		selector, err := serviceutil.InstancedPodSelector(&s)
		if err != nil {
			return nil, err
		}
		if selector.Matches(labels.Set(pod.Labels)) {
			serviceList = append(serviceList, s)
		}
	}
	return serviceList, nil
}
