package model

import (
	"testing"

	"fmt"

	"github.com/stretchr/testify/suite"
)

type CommandTestSuite struct {
	suite.Suite
}

func (suite *CommandTestSuite) TestString() {
	cmd := &Command{
		Key:     "key01",
		Type:    "type01",
		Payload: "payload01",
		Retry:   2,
	}
	suite.Equal("{key: key01, type: type01}, retried 2 times: payload01", fmt.Sprint(cmd), "error format")
}

func TestCommandTestSuite(t *testing.T) {
	suite.Run(t, new(CommandTestSuite))
}
