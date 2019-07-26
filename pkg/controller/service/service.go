package service

import (
	"context"
	"fmt"
	serviceutil "github.com/choerodon/choerodon-cluster-agent/pkg/util/service"
	"github.com/golang/glog"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubernetes/pkg/api/v1/endpoints"
	podutil "k8s.io/kubernetes/pkg/api/v1/pod"
	api "k8s.io/kubernetes/pkg/apis/core"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
)

const (
	maxRetries                         = 15
	TolerateUnreadyEndpointsAnnotation = "service.alpha.kubernetes.io/tolerate-unready-endpoints"
)

func (r *ReconcileService) checkInstancedService(service *v1.Service) error {
	selector, err := serviceutil.InstancedPodSelector(service)
	if err != nil {
		return err
	}

	pods := &v1.PodList{}
	opts := &client.ListOptions{
		LabelSelector: selector,
		Namespace:     service.Namespace,
	}
	if err := r.client.List(context.TODO(), opts, pods); err != nil {
		return err
	}

	var tolerateUnreadyEndpoints bool
	if v, ok := service.Annotations[TolerateUnreadyEndpointsAnnotation]; ok {
		b, err := strconv.ParseBool(v)
		if err == nil {
			tolerateUnreadyEndpoints = b
		} else {
			return fmt.Errorf("Failed to parse annotation %v: %v", TolerateUnreadyEndpointsAnnotation, err)
		}
	}
	subsets := []v1.EndpointSubset{}
	var totalReadyEps int = 0
	var totalNotReadyEps int = 0

	for _, pod := range pods.Items {
		if len(pod.Status.PodIP) == 0 {
			glog.V(5).Infof("Failed to find an IP for pod %s/%s", pod.Namespace, pod.Name)
			continue
		}
		if !tolerateUnreadyEndpoints && pod.DeletionTimestamp != nil {
			glog.V(5).Infof("Pod is being deleted %s/%s", pod.Namespace, pod.Name)
			continue
		}

		epa := *podToEndpointAddress(&pod)

		hostname := pod.Spec.Hostname
		if len(hostname) > 0 && pod.Spec.Subdomain == service.Name && service.Namespace == pod.Namespace {
			epa.Hostname = hostname
		}

		// Allow headless service not to have ports.
		if len(service.Spec.Ports) == 0 {
			if service.Spec.ClusterIP == api.ClusterIPNone {
				epp := v1.EndpointPort{Port: 0, Protocol: v1.ProtocolTCP}
				subsets, totalReadyEps, totalNotReadyEps = addEndpointSubset(subsets, &pod, epa, epp, tolerateUnreadyEndpoints)
			}
		} else {
			for i := range service.Spec.Ports {
				servicePort := &service.Spec.Ports[i]

				portName := servicePort.Name
				portProto := servicePort.Protocol
				portNum, err := podutil.FindPort(&pod, servicePort)
				if err != nil {
					glog.V(4).Infof("Failed to find port for service %s/%s: %v", service.Namespace, service.Name, err)
					continue
				}

				var readyEps, notReadyEps int
				epp := v1.EndpointPort{Name: portName, Port: int32(portNum), Protocol: portProto}
				subsets, readyEps, notReadyEps = addEndpointSubset(subsets, &pod, epa, epp, tolerateUnreadyEndpoints)
				totalReadyEps = totalReadyEps + readyEps
				totalNotReadyEps = totalNotReadyEps + notReadyEps
			}
		}
	}
	subsets = endpoints.RepackSubsets(subsets)

	currentEndpoints := &corev1.Endpoints{}
	epObjKey := types.NamespacedName{
		Namespace: service.Namespace,
		Name:      service.Name,
	}
	if err := r.client.Get(context.TODO(), epObjKey, currentEndpoints); err != nil {
		if errors.IsNotFound(err) {
			currentEndpoints = &v1.Endpoints{
				ObjectMeta: metav1.ObjectMeta{
					Name:      service.Name,
					Labels:    service.Labels,
					Namespace: service.Namespace,
				},
			}
		} else {
			return err
		}
	}

	createEndpoints := len(currentEndpoints.ResourceVersion) == 0

	if !createEndpoints &&
		apiequality.Semantic.DeepEqual(currentEndpoints.Subsets, subsets) &&
		apiequality.Semantic.DeepEqual(currentEndpoints.Labels, service.Labels) {
		glog.V(5).Infof("endpoints are equal for %s/%s, skipping update", service.Namespace, service.Name)
		return nil
	}
	newEndpoints := currentEndpoints.DeepCopy()
	newEndpoints.Subsets = subsets
	newEndpoints.Labels = service.Labels
	if newEndpoints.Annotations == nil {
		newEndpoints.Annotations = make(map[string]string)
	}

	glog.V(4).Infof("Update endpoints for %v/%v, ready: %d not ready: %d", service.Namespace, service.Name, totalReadyEps, totalNotReadyEps)
	if createEndpoints {
		// No previous endpoints, create them
		err = r.client.Create(context.TODO(), newEndpoints)
	} else {
		// Pre-existing
		err = r.client.Update(context.TODO(), newEndpoints)
	}
	if err != nil {
		if createEndpoints && errors.IsForbidden(err) {
			// A request is forbidden primarily for two reasons:
			// 1. namespace is terminating, endpoint creation is not allowed by default.
			// 2. policy is misconfigured, in which case no service would function anywhere.
			// Given the frequency of 1, we log at a lower level.
			glog.V(5).Infof("Forbidden from creating endpoints: %v", err)
		}
		return err
	}
	return err
}

func (r *ReconcileService) deleteInstancedService(service *corev1.Service) error {

	ep := &corev1.Endpoints{}
	epObjKey := types.NamespacedName{
		Namespace: service.Namespace,
		Name:      service.Name,
	}
	if err := r.client.Get(context.TODO(), epObjKey, ep); err != nil {
		if errors.IsNotFound(err) {
			ep = &v1.Endpoints{
				ObjectMeta: metav1.ObjectMeta{
					Name:   service.Name,
					Labels: service.Labels,
				},
			}
		} else {
			return err
		}
	}
	return r.client.Delete(context.TODO(), ep)
}

func podToEndpointAddress(pod *v1.Pod) *v1.EndpointAddress {
	return &v1.EndpointAddress{
		IP:       pod.Status.PodIP,
		NodeName: &pod.Spec.NodeName,
		TargetRef: &v1.ObjectReference{
			Kind:            "Pod",
			Namespace:       pod.ObjectMeta.Namespace,
			Name:            pod.ObjectMeta.Name,
			UID:             pod.ObjectMeta.UID,
			ResourceVersion: pod.ObjectMeta.ResourceVersion,
		}}
}

func addEndpointSubset(subsets []v1.EndpointSubset, pod *v1.Pod, epa v1.EndpointAddress,
	epp v1.EndpointPort, tolerateUnreadyEndpoints bool) ([]v1.EndpointSubset, int, int) {
	var readyEps int = 0
	var notReadyEps int = 0
	if tolerateUnreadyEndpoints || podutil.IsPodReady(pod) {
		subsets = append(subsets, v1.EndpointSubset{
			Addresses: []v1.EndpointAddress{epa},
			Ports:     []v1.EndpointPort{epp},
		})
		readyEps++
	} else if shouldPodBeInEndpoints(pod) {
		glog.V(5).Infof("Pod is out of service: %v/%v", pod.Namespace, pod.Name)
		subsets = append(subsets, v1.EndpointSubset{
			NotReadyAddresses: []v1.EndpointAddress{epa},
			Ports:             []v1.EndpointPort{epp},
		})
		notReadyEps++
	}
	return subsets, readyEps, notReadyEps
}

func shouldPodBeInEndpoints(pod *v1.Pod) bool {
	switch pod.Spec.RestartPolicy {
	case v1.RestartPolicyNever:
		return pod.Status.Phase != v1.PodFailed && pod.Status.Phase != v1.PodSucceeded
	case v1.RestartPolicyOnFailure:
		return pod.Status.Phase != v1.PodSucceeded
	default:
		return true
	}
}
