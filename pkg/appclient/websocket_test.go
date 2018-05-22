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
	"github.com/stretchr/testify/suite"
)

type WebsocketTestSuite struct {
	suite.Suite
	upgrader  websocket.Upgrader
	server    *httptest.Server
	serverURL *url.URL
}

func (suite *WebsocketTestSuite) SetupSuite() {
	suite.upgrader = websocket.Upgrader{}
	suite.server = httptest.NewServer(suite.router())
	suite.serverURL, _ = url.Parse(fmt.Sprintf("ws%s", strings.TrimPrefix(suite.server.URL, "http")))
}

func (suite *WebsocketTestSuite) TearDownSuite() {
	suite.server.Close()
}

func (suite *WebsocketTestSuite) router() http.Handler {
	router := gin.Default()
	router.GET("/", func(c *gin.Context) {
		suite.echo(c.Writer, c.Request)
	})
	return router
}

func (suite *WebsocketTestSuite) echo(w http.ResponseWriter, r *http.Request) {
	conn, err := suite.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	for {
		mt, message, err := conn.ReadMessage()
		if err != nil {
			suite.Equal(true, IsExpectedWSCloseError(err), "no unexpected error read message")
			return
		}
		err = conn.WriteMessage(mt, message)
		suite.Nil(err, "no error write message")
	}
}

func (suite *WebsocketTestSuite) TestDial() {
	s := httptest.NewServer(http.HandlerFunc(suite.echo))
	defer s.Close()

	conn, err := dial(suite.serverURL.String(), Token("token"))
	suite.Nil(err, "no error websocket dial")
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
		suite.Nil(err, "no error write message")
		_, p, err := conn.ReadMessage()
		suite.Nil(err, "no error read message")
		suite.Equal(test.want, string(p), "bad message")
	}
}

func TestWebsocketTestSuit(t *testing.T) {
	suite.Run(t, new(WebsocketTestSuite))
}
