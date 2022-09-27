package action

import (
	"context"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-hand/helm/pkg/agent/model"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes"
	"strconv"
)

func AddLabel(imagePullSecret []v1.LocalObjectReference,
	command int64,
	V1Command string,
	appServiceId int64,
	V1AppServiceId string,
	info *resource.Info,
	commit, version, releaseName, chartName, agentVersion, testLabel, namespace string,
	isTest bool,
	isUpgrade bool,
	clientSet *kubernetes.Clientset) error {
	t := info.Object.(*unstructured.Unstructured)
	kind := info.Mapping.GroupVersionKind.Kind

	l := t.GetLabels()

	if l == nil {
		l = make(map[string]string)
	}

	var addBaseLabels = func() {
		l[model.ReleaseLabel] = releaseName
		l[model.AgentVersionLabel] = agentVersion
		l[model.CommitLabel] = commit
	}
	var addAppLabels = func() {
		l[model.AppLabel] = chartName
		l[model.AppVersionLabel] = version
	}

	var addTemplateAppLabels = func() {
		tplLabels := getTemplateLabels(t.Object)
		tplLabels[model.ReleaseLabel] = releaseName
		tplLabels[model.AgentVersionLabel] = agentVersion
		tplLabels[model.CommitLabel] = commit
		//12.05 新增打标签。
		//0 表示的是安装未填入值 -1代表更新
		if (appServiceId != 0 && appServiceId != -1) || (V1AppServiceId != "0" && V1AppServiceId != "-1") {
			tplLabels[model.AppServiceIdLabel] = strconv.FormatInt(appServiceId, 10)
			tplLabels[model.V1AppServiceIdLabel] = V1AppServiceId
		}
		if !isTest {
			tplLabels[model.CommandLabel] = strconv.Itoa(int(command))
			tplLabels[model.V1CommandLabel] = V1Command
		}
		tplLabels[model.AppLabel] = chartName
		tplLabels[model.AppVersionLabel] = version
		if err := setTemplateLabels(t.Object, tplLabels); err != nil {
			glog.Warningf("Set Template Labels failed, %v", err)
		}
	}
	var addSelectorAppLabels = func() {
		selectorLabels, _, err := unstructured.NestedStringMap(t.Object, "spec", "selector", "matchLabels")
		if err != nil {
			glog.Warningf("Get Selector Labels failed, %v", err)
		}
		if selectorLabels == nil {
			selectorLabels = make(map[string]string)
		}
		selectorLabels[model.ReleaseLabel] = releaseName
		if err := unstructured.SetNestedStringMap(t.Object, selectorLabels, "spec", "selector", "matchLabels"); err != nil {
			glog.Warningf("Set Selector label failed, %v", err)
		}
	}

	// add private image pull secrets
	var addImagePullSecrets = func() {
		secrets, _, err := nestedLocalObjectReferences(t.Object, "spec", "template", "spec", "imagePullSecrets")
		if err != nil {
			glog.Warningf("Get ImagePullSecrets failed, %v", err)
		}
		if secrets == nil {
			secrets = make([]v1.LocalObjectReference, 0)

		}
		secrets = append(secrets, imagePullSecret...)
		// SetNestedField method just support a few types
		s := make([]interface{}, 0)
		for _, secret := range secrets {
			m := make(map[string]interface{})
			m["name"] = secret.Name
			s = append(s, m)
		}
		if err := unstructured.SetNestedField(t.Object, s, "spec", "template", "spec", "imagePullSecrets"); err != nil {
			glog.Warningf("Set ImagePullSecrets failed, %v", err)
		}
	}

	switch kind {
	case "ReplicationController", "ReplicaSet", "Deployment":
		addAppLabels()
		addTemplateAppLabels()
		addSelectorAppLabels()
		addImagePullSecrets()
		if isUpgrade {
			if kind == "ReplicaSet" {
				rs, err := clientSet.AppsV1().ReplicaSets(namespace).Get(context.Background(), t.GetName(), metav1.GetOptions{})
				if errors.IsNotFound(err) {
					break
				}
				if err != nil {
					glog.Warningf("Failed to get ReplicaSet,error is %s.", err.Error())
					return err
				}
				err = setReplicas(t.Object, int64(*rs.Spec.Replicas))
				if err != nil {
					glog.Warningf("Failed to set replicas,error is %s", err.Error())
					return err
				}
			}
			if kind == "Deployment" {
				dp, err := clientSet.AppsV1().Deployments(namespace).Get(context.Background(), t.GetName(), metav1.GetOptions{})
				if errors.IsNotFound(err) {
					break
				}
				if err != nil {
					glog.Warningf("Failed to get ReplicaSet,error is %s.", err.Error())
					return err
				}
				err = setReplicas(t.Object, int64(*dp.Spec.Replicas))
				if err != nil {
					glog.Warningf("Failed to set replicas,error is %s", err.Error())
					return err
				}
			}
		}
	case "ConfigMap":
	case "Service":
		l[model.NetworkLabel] = "service"
		l[model.NetworkNoDelLabel] = "true"
	case "Ingress":
		l[model.NetworkLabel] = "ingress"
		l[model.NetworkNoDelLabel] = "true"
	case "Job":
		addImagePullSecrets()
		tplLabels := getTemplateLabels(t.Object)
		if isTest {
			l[model.TestLabel] = testLabel
			tplLabels[model.TestLabel] = testLabel
			tplLabels[model.ReleaseLabel] = releaseName
		}
		tplLabels[model.CommitLabel] = commit
		if err := setTemplateLabels(t.Object, tplLabels); err != nil {
			glog.Warningf("Set Test-Template Labels failed, %v", err)
		}
	case "DaemonSet", "StatefulSet":
		addAppLabels()
		addTemplateAppLabels()
		addImagePullSecrets()
		if isUpgrade {
			if kind == "StatefulSet" {
				sts, err := clientSet.AppsV1().StatefulSets(namespace).Get(context.Background(), t.GetName(), metav1.GetOptions{})
				if errors.IsNotFound(err) {
					break
				}
				if err != nil {
					glog.Warningf("Failed to get ReplicaSet,error is %s.", err.Error())
					return err
				}
				err = setReplicas(t.Object, int64(*sts.Spec.Replicas))
				if err != nil {
					glog.Warningf("Failed to set replicas,error is %s", err.Error())
					return err
				}
			}
		}
	case "Secret":
		addAppLabels()
	case "Pod":
		addAppLabels()
	default:
		glog.Warningf("Skipping to add choerodon label, object: Kind %s of Release %s", kind, releaseName)
		return nil
	}
	if t.GetNamespace() != "" && t.GetNamespace() != namespace && chartName != "prometheus-operator" {
		return fmt.Errorf(" Kind:%s Name:%s. The namespace of this resource is not consistent with helm release", kind, t.GetName())
	}
	// add base labels
	addBaseLabels()
	t.SetLabels(l)

	annotations := t.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[model.CommitLabel] = commit
	t.SetAnnotations(annotations)
	return nil
}

func setTemplateLabels(obj map[string]interface{}, templateLabels map[string]string) error {
	return unstructured.SetNestedStringMap(obj, templateLabels, "spec", "template", "metadata", "labels")
}

func setReplicas(obj map[string]interface{}, value int64) error {
	return unstructured.SetNestedField(obj, value, "spec", "replicas")
}

func getTemplateLabels(obj map[string]interface{}) map[string]string {
	tplLabels, _, err := unstructured.NestedStringMap(obj, "spec", "template", "metadata", "labels")
	if err != nil {
		glog.Warningf("Get Template Labels failed, %v", err)
	}
	if tplLabels == nil {
		tplLabels = make(map[string]string)
	}
	return tplLabels
}

func nestedLocalObjectReferences(obj map[string]interface{}, fields ...string) ([]v1.LocalObjectReference, bool, error) {
	val, found, err := unstructured.NestedFieldNoCopy(obj, fields...)
	if !found || err != nil {
		return nil, found, err
	}

	m, ok := val.([]v1.LocalObjectReference)
	if ok {
		return m, true, nil
		//return nil, false, fmt.Errorf("%v accessor error: %v is of the type %T, expected []v1.LocalObjectReference", strings.Join(fields, "."), val, val)
	}

	if m, ok := val.([]interface{}); ok {
		secrets := make([]v1.LocalObjectReference, 0)
		for _, v := range m {
			if vv, ok := v.(map[string]interface{}); ok {
				v2 := vv["name"]
				secret := v1.LocalObjectReference{}
				if secret.Name, ok = v2.(string); ok {
					secrets = append(secrets, secret)
				}
			}
		}
		return secrets, true, nil
	}
	return m, true, nil
}
