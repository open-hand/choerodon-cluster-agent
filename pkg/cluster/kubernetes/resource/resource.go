// Copyright 2016 Weaveworks Ltd.
// Use of this source code is governed by a Apache License Version 2.0 license
// that can be found at https://github.com/weaveworks/flux/blob/master/LICENSE

package resource

import (
	"gopkg.in/yaml.v2"

	"github.com/choerodon/choerodon-cluster-agent/pkg/util/errors"
	"github.com/choerodon/choerodon-cluster-agent/pkg/util/resource"
)

// -- unmarshaling code for specific object and field types

// struct to embed in objects, to provide default implementation
type BaseObject struct {
	SourceName string
	BytesArray []byte
	Kind       string        `yaml:"kind"`
	Meta       resource.Meta `yaml:"metadata"`
}

func (o BaseObject) ResourceID() resource.ResourceID {
	ns := o.Meta.Namespace
	if ns == "" {
		ns = "default"
	}
	return resource.MakeResourceID(ns, o.Kind, o.Meta.Name)
}

// It's useful for comparisons in tests to be able to remove the
// record of bytes
func (o *BaseObject) debyte() {
	o.BytesArray = nil
}

func (o BaseObject) Source() string {
	return o.SourceName
}

func (o BaseObject) SourceKind() string {
	return o.Kind
}

func (o BaseObject) Bytes() []byte {
	return o.BytesArray
}

func (o BaseObject) Metas() resource.Meta {
	return o.Meta
}

func unmarshalObject(namespace string, source string, bytes []byte) (resource.Resource, error) {
	var base = BaseObject{SourceName: source, BytesArray: bytes}
	if err := yaml.Unmarshal(bytes, &base); err != nil {
		return nil, err
	}
	base.Meta.Namespace = namespace
	return base, nil
}

type rawList struct {
	Items []map[string]interface{}
}

func unmarshalList(namespace string, base BaseObject, raw *rawList, list *List) error {
	list.BaseObject = base
	list.Items = make([]resource.Resource, len(raw.Items), len(raw.Items))
	for i, item := range raw.Items {
		bytes, err := yaml.Marshal(item)
		if err != nil {
			return err
		}
		res, err := unmarshalObject(namespace, base.SourceName, bytes)
		if err != nil {
			return err
		}
		list.Items[i] = res
	}
	return nil
}

func makeUnmarshalObjectErr(source string, err error) *errors.Error {
	return &errors.Error{
		Type: errors.User,
		Err:  err,
		Help: `Could not parse "` + source + `".

This likely means it is malformed YAML.
`,
	}
}

// For reference, the Kubernetes v1 types are in:
// https://github.com/kubernetes/client-go/blob/master/pkg/api/v1/types.go
