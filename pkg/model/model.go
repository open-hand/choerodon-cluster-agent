package model

import (
	"fmt"
	"strings"
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
	keyValues := strings.Split(key, ".")
	for _, keyValue := range keyValues {
		if strings.Contains(keyValue, "env") {
			return strings.Split(keyValue, ":")[1]
		}
	}
	return ""
}
