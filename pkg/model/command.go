package model

import (
	"fmt"
)

type Command struct {
	Key     string `json:"key,omitempty"`
	Type    string `json:"type,omitempty"`
	Payload string `json:"payload,omitempty"`
	Retry   uint   `json:"-"`
}

func (c *Command) String() string {
	return fmt.Sprintf("{key: %s, type: %s}, retried %d times: %s", c.Key, c.Type, c.Retry, c.Payload)
}
