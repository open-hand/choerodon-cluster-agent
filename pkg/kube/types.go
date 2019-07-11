package kube

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type CommonObject struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Spec defines the behavior of a service.
	// https://git.k8s.io/community/contributors/devel/api-conventions.md#spec-and-status
	// +optional
	Spec interface{} `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`

	// Most recently observed status of the service.
	// Populated by the system.
	// Read-only.
	// More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#spec-and-status
	// +optional
	Status interface{} `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

func (c *CommonObject) GetName() string {
	return ""
}
func (c *CommonObject) GetNamespace() string {
	return ""
}
func (c *CommonObject) GetLabels() string {
	return ""
}
func (c *CommonObject) GetAnnotations() string {
	return ""
}

func (c *CommonObject) GetObjectKind() schema.ObjectKind {
	return nil
}

func (c *CommonObject) DeepCopyObject() runtime.Object {
	return c
}
