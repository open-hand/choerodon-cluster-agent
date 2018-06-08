package appclient

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
)

var (
	websocketTestUpgrader = websocket.Upgrader{}
)

func websocketTestRouter(t *testing.T) http.Handler {
	router := gin.Default()
	router.GET("/", func(c *gin.Context) {
		websocketTestEcho(t, c.Writer, c.Request)
	})
	return router
}

func websocketTestEcho(t *testing.T, w http.ResponseWriter, r *http.Request) {
	conn, err := websocketTestUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	for {
		mt, message, err := conn.ReadMessage()
		if err != nil {
			assert.Equal(t, true, IsExpectedWSCloseError(err), "no unexpected error read message")
			return
		}
		err = conn.WriteMessage(mt, message)
		assert.Nil(t, err, "no error write message")
	}
}

func TestDial(t *testing.T) {
	server := httptest.NewServer(websocketTestRouter(t))
	defer server.Close()
	serverURL, _ := url.Parse(fmt.Sprintf("ws%s", strings.TrimPrefix(server.URL, "http")))
	conn, err := dial(serverURL.String(), Token("token"))
	assert.Nil(t, err, "no error websocket dial")
	defer conn.Close()

	tests := []struct {
		input string
		want  string
	}{
		{"hello", "hello"},
		{"a1b2c", "a1b2c"},
		{"123", "123"},
	}

	for _, test := range tests {
		err := conn.WriteMessage(websocket.TextMessage, []byte(test.input))
		assert.Nil(t, err, "no error write message")
		_, p, err := conn.ReadMessage()
		assert.Nil(t, err, "no error read message")
		assert.Equal(t, test.want, string(p), "bad message")
	}
}
