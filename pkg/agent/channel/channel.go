package channel

import "github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"

type CRChan struct {
	CommandChan  chan *model.Packet
	ResponseChan chan *model.Packet
}

func NewCRChannel(commandSize, RespSize int) *CRChan {
	return &CRChan{
		CommandChan:  make(chan *model.Packet, commandSize),
		ResponseChan: make(chan *model.Packet, RespSize),
	}
}

func (crChannel *CRChan) CurrentQueueSize() (int, int) {
	return len(crChannel.CommandChan), len(crChannel.ResponseChan)
}
