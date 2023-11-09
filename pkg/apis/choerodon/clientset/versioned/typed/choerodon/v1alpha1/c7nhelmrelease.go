/*
Copyright 2018 Jetstack Ltd.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package v1alpha1

import (
	"context"
	"github.com/choerodon/choerodon-cluster-agent/pkg/apis/choerodon/clientset/versioned/scheme"
	"github.com/choerodon/choerodon-cluster-agent/pkg/apis/choerodon/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
)

// CertificatesGetter has a method to return a CertificateInterface.
// A group's client should implement this interface.
type C7nHelmReleaseGetter interface {
	C7nHelmReleases(namespace string) C7nHelmReleaseInterface
}

// CertificateInterface has methods to work with Certificate resources.
type C7nHelmReleaseInterface interface {
	Create(ctx context.Context, c7NHelmRelease *v1alpha1.C7NHelmRelease) (*v1alpha1.C7NHelmRelease, error)
	Update(ctx context.Context, c7NHelmRelease *v1alpha1.C7NHelmRelease) (*v1alpha1.C7NHelmRelease, error)
	UpdateStatus(ctx context.Context, c7NHelmRelease *v1alpha1.C7NHelmRelease) (*v1alpha1.C7NHelmRelease, error)
	Delete(ctx context.Context, name string, options *v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(ctx context.Context, name string, options v1.GetOptions) (*v1alpha1.C7NHelmRelease, error)
	List(ctx context.Context, opts v1.ListOptions) (*v1alpha1.C7NHelmReleaseList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.C7NHelmRelease, err error)
}

// c7NHelmReleases implements C7NHelmReleaseInterface
type c7NHelmReleases struct {
	client rest.Interface
	ns     string
}

// newC7NHelmReleases returns a C7NHelmReleases
func newC7NHelmReleases(c *C7nHelmReleaseGetterV1alpha1Client, namespace string) *c7NHelmReleases {
	return &c7NHelmReleases{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the c7NHelmRelease, and returns the corresponding c7NHelmRelease object, and an error if there is any.
func (c *c7NHelmReleases) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.C7NHelmRelease, err error) {
	result = &v1alpha1.C7NHelmRelease{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("C7NHelmReleases").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of C7NHelmReleases that match those selectors.
func (c *c7NHelmReleases) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.C7NHelmReleaseList, err error) {
	result = &v1alpha1.C7NHelmReleaseList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("C7NHelmReleases").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested c7NHelmReleases.
func (c *c7NHelmReleases) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("C7NHelmReleases").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch(ctx)
}

// Create takes the representation of a c7NHelmRelease and creates it.  Returns the server's representation of the c7NHelmRelease, and an error, if there is any.
func (c *c7NHelmReleases) Create(ctx context.Context, c7NHelmRelease *v1alpha1.C7NHelmRelease) (result *v1alpha1.C7NHelmRelease, err error) {
	result = &v1alpha1.C7NHelmRelease{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("C7NHelmReleases").
		Body(c7NHelmRelease).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a c7NHelmRelease and updates it. Returns the server's representation of the c7NHelmRelease, and an error, if there is any.
func (c *c7NHelmReleases) Update(ctx context.Context, c7NHelmRelease *v1alpha1.C7NHelmRelease) (result *v1alpha1.C7NHelmRelease, err error) {
	result = &v1alpha1.C7NHelmRelease{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("C7NHelmReleases").
		Name(c7NHelmRelease.Name).
		Body(c7NHelmRelease).
		Do(ctx).
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().

func (c *c7NHelmReleases) UpdateStatus(ctx context.Context, c7NHelmRelease *v1alpha1.C7NHelmRelease) (result *v1alpha1.C7NHelmRelease, err error) {
	result = &v1alpha1.C7NHelmRelease{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("C7NHelmReleases").
		Name(c7NHelmRelease.Name).
		SubResource("status").
		Body(c7NHelmRelease).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the c7NHelmRelease and deletes it. Returns an error if one occurs.
func (c *c7NHelmReleases) Delete(ctx context.Context, name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("C7NHelmReleases").
		Name(name).
		Body(options).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *c7NHelmReleases) DeleteCollection(ctx context.Context, options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("C7NHelmReleases").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched c7NHelmRelease.
func (c *c7NHelmReleases) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.C7NHelmRelease, err error) {
	result = &v1alpha1.C7NHelmRelease{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("C7NHelmReleases").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
