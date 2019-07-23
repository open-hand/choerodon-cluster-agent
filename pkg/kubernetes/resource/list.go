// Copyright 2016 Weaveworks Ltd.
// Use of this source code is governed by a Apache License Version 2.0 license
// that can be found at https://github.com/weaveworks/flux/blob/master/LICENSE

package resource

import "github.com/choerodon/choerodon-cluster-agent/pkg/util/resource"

type List struct {
	BaseObject
	Items []resource.Resource
}
