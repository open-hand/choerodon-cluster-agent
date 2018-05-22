package worker

import (
	"github.com/golang/glog"

	"github.com/choerodon/choerodon-agent/pkg/model"
)

func NewResponseError(key string, cmdType string, err error) *model.Response {
	glog.Error(err)
	return &model.Response{
		Key:     key,
		Type:    cmdType,
		Payload: err.Error(),
	}
}
