// Copyright 2016 Weaveworks Ltd.
// Use of this source code is governed by a Apache License Version 2.0 license
// that can be found at https://github.com/weaveworks/flux/blob/master/LICENSE

package resource

import (
	yaml "gopkg.in/yaml.v2"

	c7n_error "github.com/choerodon/choerodon-agent/pkg/errors"
	"github.com/choerodon/choerodon-agent/pkg/resource"
)

// -- unmarshaling code for specific object and field types

// struct to embed in objects, to provide default implementation
type baseObject struct {
	source string
	bytes  []byte
	Kind   string `yaml:"kind"`
	Meta   struct {
		Namespace   string            `yaml:"namespace"`
		Name        string            `yaml:"name"`
		Annotations map[string]string `yaml:"annotations,omitempty"`
	} `yaml:"metadata"`
}

func (o baseObject) ResourceID() resource.ResourceID {
	ns := o.Meta.Namespace
	if ns == "" {
		ns = "default"
	}
	return resource.MakeResourceID(ns, o.Kind, o.Meta.Name)
}

// It's useful for comparisons in tests to be able to remove the
// record of bytes
func (o *baseObject) debyte() {
	o.bytes = nil
}

func (o baseObject) Source() string {
	return o.source
}

func (o baseObject) Bytes() []byte {
	return o.bytes
}

func unmarshalObject(namespace string, source string, bytes []byte) (resource.Resource, error) {
	var base = baseObject{source: source, bytes: bytes}
	if err := yaml.Unmarshal(bytes, &base); err != nil {
		return nil, err
	}
	base.Meta.Namespace = namespace
	return base, nil
}

type rawList struct {
	Items []map[string]interface{}
}

func unmarshalList(namespace string, base baseObject, raw *rawList, list *List) error {
	list.baseObject = base
	list.Items = make([]resource.Resource, len(raw.Items), len(raw.Items))
	for i, item := range raw.Items {
		bytes, err := yaml.Marshal(item)
		if err != nil {
			return err
		}
		res, err := unmarshalObject(namespace, base.source, bytes)
		if err != nil {
			return err
		}
		list.Items[i] = res
	}
	return nil
}

func makeUnmarshalObjectErr(source string, err error) *c7n_error.Error {
	return &c7n_error.Error{
		Type: c7n_error.User,
		Err:  err,
		Help: `Could not parse "` + source + `".

This likely means it is malformed YAML.
`,
	}
}

// For reference, the Kubernetes v1 types are in:
// https://github.com/kubernetes/client-go/blob/master/pkg/api/v1/types.go
