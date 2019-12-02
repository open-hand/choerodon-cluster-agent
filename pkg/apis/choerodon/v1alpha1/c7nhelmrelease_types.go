package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// C7NHelmReleaseSpec defines the desired state of C7NHelmRelease
// +k8s:openapi-gen=true
type C7NHelmReleaseSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html
	RepoURL          string                        `json:"repoUrl,omitempty"`
	ChartName        string                        `json:"chartName,omitempty"`
	ChartVersion     string                        `json:"chartVersion,omitempty"`
	CommandId        int                           `json:"commandId,omitempty"`
	Values           string                        `json:"values,omitempty"`
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
    //12.01新增字段，Deployment要打的label多加一个choerodon.io/app-service-id
	AppServiceId int64 `json:"appServiceId,omitempty"`

}


// C7NHelmReleaseStatus defines the observed state of C7NHelmRelease
// +k8s:openapi-gen=true
type C7NHelmReleaseStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// C7NHelmRelease is the Schema for the c7nhelmreleases API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
type C7NHelmRelease struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   C7NHelmReleaseSpec   `json:"spec,omitempty"`
	Status C7NHelmReleaseStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// C7NHelmReleaseList contains a list of C7NHelmRelease
type C7NHelmReleaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []C7NHelmRelease `json:"items"`
}

func init() {
	SchemeBuilder.Register(&C7NHelmRelease{}, &C7NHelmReleaseList{})
}
