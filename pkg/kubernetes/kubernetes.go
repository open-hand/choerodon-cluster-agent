// Copyright 2016 Weaveworks Ltd.
// Use of this source code is governed by a Apache License Version 2.0 license
// that can be found at https://github.com/weaveworks/flux/blob/master/LICENSE

package kubernetes

import (
	"bytes"
	"github.com/choerodon/choerodon-cluster-agent/pkg/crd_client/certificate/client/clientset/versioned"
	"github.com/choerodon/choerodon-cluster-agent/pkg/crd_client/certificate/client/clientset/versioned/typed/certmanager/v1alpha1"
	operatorutil "github.com/choerodon/choerodon-cluster-agent/pkg/util/operator"
	"sync"

	k8syaml "github.com/ghodss/yaml"
	"gopkg.in/yaml.v2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sclient "k8s.io/client-go/kubernetes"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	v1beta1extensions "k8s.io/client-go/kubernetes/typed/extensions/v1beta1"

	"github.com/choerodon/choerodon-cluster-agent/pkg/util/resource"
)

type extendedClient struct {
	v1core.CoreV1Interface
	v1beta1extensions.ExtensionsV1beta1Interface
	v1alpha1.CertmanagerV1alpha1Interface
}

type ChangeSet struct {
	objs map[string][]*ApiObject
}

func (cs *ChangeSet) DeleteObj() []*ApiObject {
	return cs.objs["delete"]
}

func (cs *ChangeSet) ApplyObj() []*ApiObject {
	return cs.objs["apply"]
}

func makeChangeSet() ChangeSet {
	return ChangeSet{objs: make(map[string][]*ApiObject)}
}

func (cs *ChangeSet) stage(cmd string, o *ApiObject) {
	cs.objs[cmd] = append(cs.objs[cmd], o)
}

// Applier is something that will apply a changeset to the cluster.
type Applier interface {
	Apply(namespace string, cs ChangeSet) SyncError
}

type Describer interface {
	Describe(namespace, sourceKind, sourceName string) string
}

// Cluster is a handle to a Kubernetes API server.
// (Typically, this code is deployed into the same cluster.)
type Cluster struct {
	client    extendedClient
	mgrs      *operatorutil.MgrList
	applier   Applier
	describer Describer
	mu        sync.Mutex
}

// NewCluster returns a usable cluster.
func NewCluster(
	clientset k8sclient.Interface,
	crdClientSet versioned.Interface,
	mgrs *operatorutil.MgrList,
	applier Applier,
	describer Describer) *Cluster {

	c := &Cluster{
		client: extendedClient{
			clientset.CoreV1(),
			clientset.ExtensionsV1beta1(),
			crdClientSet.Certmanager(),
		},
		mgrs:      mgrs,
		applier:   applier,
		describer: describer,
	}

	return c
}

// Export exports cluster resources
func (c *Cluster) Export(namespace string) ([]byte, error) {
	var config bytes.Buffer

	for _, resourceKind := range resourceKinds {
		resources, err := resourceKind.getResources(c, namespace)
		if err != nil {
			if se, ok := err.(*apierrors.StatusError); ok && se.ErrStatus.Reason == meta_v1.StatusReasonNotFound {
				// Kind not supported by API server, skip
				continue
			} else {
				return nil, err
			}
		}

		for _, r := range resources {
			if !isAgent(r, namespace) {
				if err := appendYAML(&config, r.apiVersion, r.kind, r.k8sObject); err != nil {
					return nil, err
				}
			}
		}
	}
	return config.Bytes(), nil
}

func (c *Cluster) DescribeResource(namespace, sourceKind, sourceName string) string {
	return c.describer.Describe(namespace, sourceKind, sourceName)
}

// Sync performs the given actions on resources. Operations are
// asynchronous, but serialised.
func (c *Cluster) Sync(namespace string, spec SyncDef) error {
	cs := makeChangeSet()
	var errs SyncError
	for _, action := range spec.Actions {
		stages := []struct {
			res resource.Resource
			cmd string
		}{
			{action.Delete, "delete"},
			{action.Apply, "apply"},
		}
		for _, stage := range stages {
			if stage.res == nil {
				continue
			}
			obj, err := parseObj(stage.res.Bytes())
			if err == nil {
				obj.Resource = stage.res
				cs.stage(stage.cmd, obj)
			} else {
				errs = append(errs, ResourceError{Resource: stage.res, Error: err})
				break
			}
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if applyErrs := c.applier.Apply(namespace, cs); len(applyErrs) > 0 {
		errs = append(errs, applyErrs...)
	}

	// If `nil`, errs is a cluster.SyncError(nil) rather than error(nil)
	if errs != nil {
		return errs
	}
	return nil
}

// --- internal types for keeping track of syncing

type metadata struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
}

type ApiObject struct {
	resource.Resource
	Kind     string   `yaml:"kind"`
	Metadata metadata `yaml:"metadata"`
}

// A convenience for getting an minimal object from some bytes.
func parseObj(def []byte) (*ApiObject, error) {
	obj := ApiObject{}
	return &obj, yaml.Unmarshal(def, &obj)
}

func (o *ApiObject) hasNamespace() bool {
	return o.Metadata.Namespace != ""
}

// Kubernetes has a mechanism of "Add-ons", whereby manifest files
// left in a particular directory on the Kubernetes master will be
// applied. We can recognise these, because they:
//  1. Must be in the namespace `kube-system`; and,
//  2. Must have one of the labels below set, else the addon manager will ignore them.
//
// We want to ignore add-ons, since they are managed by the add-on
// manager, and attempts to control them via other means will fail.

// k8sObject represents an value from which you can obtain typical
// Kubernetes metadata. These methods are implemented by the
// Kubernetes API resource types.
type k8sObject interface {
	GetName() string
	GetNamespace() string
	GetLabels() map[string]string
	GetAnnotations() map[string]string
}

func isAgent(obj k8sObject, name string) bool {
	return obj.GetName() == name
}

// kind & apiVersion must be passed separately as the object's TypeMeta is not populated
func appendYAML(buffer *bytes.Buffer, apiVersion, kind string, object interface{}) error {
	yamlBytes, err := k8syaml.Marshal(object)
	if err != nil {
		return err
	}
	buffer.WriteString("---\n")
	buffer.WriteString("apiVersion: ")
	buffer.WriteString(apiVersion)
	buffer.WriteString("\nkind: ")
	buffer.WriteString(kind)
	buffer.WriteString("\n")
	buffer.Write(yamlBytes)
	return nil
}
