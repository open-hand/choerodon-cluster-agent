package model

import (
	"fmt"
)

type Packet struct {
	Key     string `json:"key,omitempty"`
	Type    string `json:"type,omitempty"`
	Payload string `json:"payload,omitempty"`
}




func (c *Packet) String() string {
	return fmt.Sprintf("{key: %s, type: %s}: %s", c.Key, c.Type, c.Payload)
}


func (c *Packet) Namespace() string {
	key := c.Key
	//todo
	//从key中取出命名空间
	return key
}


