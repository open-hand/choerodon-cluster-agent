package model

import (
	"fmt"
)

type Command struct {
	Key     string `json:"key,omitempty"`
	Type    string `json:"type,omitempty"`
	Payload string `json:"payload,omitempty"`
}

func (c *Command) String() string {
	return fmt.Sprintf("{key: %s, type: %s}: %s", c.Key, c.Type, c.Payload)
}
