package worker

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewResponseError(t *testing.T) {
	key, cmdType, errorMsg := "key", "type", "error message"
	resp := NewResponseError(key, cmdType, fmt.Errorf(errorMsg))
	assert.Equal(t, key, resp.Key, "error response")
	assert.Equal(t, cmdType, resp.Type, "error response")
	assert.Equal(t, errorMsg, resp.Payload, "error response")
}
