package model

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommandString(t *testing.T) {
	cmd := &Packet{
		Key:     "key01",
		Type:    "type01",
		Payload: "payload01",
	}
	assert.Equal(t, "{key: key01, type: type01}: payload01", fmt.Sprint(cmd), "error format")
}