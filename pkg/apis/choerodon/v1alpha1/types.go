package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type C7NHelmReleaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []C7NHelmRelease `json:"items"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type C7NHelmRelease struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              C7NHelmReleaseSpec   `json:"spec"`
	Status            C7NHelmReleaseStatus `json:"status,omitempty"`
}

type C7NHelmReleaseSpec struct {
	RepoURL          string                        `json:"repoURL,omitempty"`
	ChartName        string                        `json:"chartName,omitempty"`
	ChartVersion     string                        `json:"chartVersion,omitempty"`
	Values           string                        `json:"values,omitempty"`
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
}

type C7NHelmReleaseStatus struct {
}
