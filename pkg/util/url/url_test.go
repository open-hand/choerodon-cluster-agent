package url

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseURL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"ws://localhost:8060/agent/?key=env:choerodon-agent-test", "ws://localhost:8060/agent/log?key=env:choerodon-agent-test"},
		{"ws://localhost:8060/agent/", "ws://localhost:8060/agent/log"},
	}
	for _, test := range tests {
		base, _ := url.Parse(test.input)
		newURL, err := ParseURL(base, "log")
		assert.Nil(t, err, "no error parse url")
		assert.Equal(t, test.want, newURL.String(), "error parse url")
	}
}
