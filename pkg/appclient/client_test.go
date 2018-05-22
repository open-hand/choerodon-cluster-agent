package appclient

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/suite"

	"github.com/choerodon/choerodon-agent/pkg/model"
	model_helm "github.com/choerodon/choerodon-agent/pkg/model/helm"
)

type ClientTestSuite struct {
	suite.Suite
	upgrader     websocket.Upgrader
	commandChan  chan *model.Command
	responseChan chan *model.Response
	stopCh       chan struct{}
	server       *httptest.Server
	serverURL    *url.URL
}

func (suite *ClientTestSuite) SetupSuite() {
	suite.upgrader = websocket.Upgrader{}
	suite.server = httptest.NewServer(suite.router())
	suite.serverURL, _ = url.Parse(fmt.Sprintf("ws%s/agent", strings.TrimPrefix(suite.server.URL, "http")))
}

func (suite *ClientTestSuite) TearDownSuite() {
	suite.server.Close()
}

func (suite *ClientTestSuite) router() http.Handler {
	router := gin.Default()
	router.GET("/agent", func(c *gin.Context) {
		suite.serveWs(c.Writer, c.Request)
	})
	return router
}

func (suite *ClientTestSuite) serveWs(w http.ResponseWriter, r *http.Request) {
	conn, err := suite.upgrader.Upgrade(w, r, nil)
	suite.Nil(err, "no error upgrades")
	defer conn.Close()

	helmInstallMessage := &model_helm.InstallReleaseRequest{
		RepoURL:      "http://charts.test.com",
		ChartName:    "test-service",
		ChartVersion: "0.1.0",
		Values:       "",
		ReleaseName:  "test-test-service",
	}

	var commandMsgPayloadBytes []byte
	commandMsgPayloadBytes, err = json.Marshal(helmInstallMessage)
	command := &model.Command{
		Key:     "helm:release:install",
		Type:    model.HelmInstallRelease,
		Payload: string(commandMsgPayloadBytes),
	}

	err = conn.WriteJSON(command)
	suite.Nil(err, "no error write json")

	_, _, err = conn.ReadMessage()
	suite.Nil(err, "no error read message")

	conn.WriteMessage(websocket.CloseMessage, []byte{})
}

func (suite *ClientTestSuite) TestClient() {
	suite.commandChan = make(chan *model.Command)
	suite.responseChan = make(chan *model.Response)
	suite.stopCh = make(chan struct{})

	c, err := NewClient(Token("token"), suite.serverURL.String(), suite.commandChan, suite.responseChan)
	suite.Nil(err, "no error create new client")
	defer c.Stop()

	go c.Start(suite.stopCh)

	cmd := <-suite.commandChan
	suite.Equal("helm:release:install", cmd.Key, "Bad command")

	helmInstallResp := &model_helm.InstallReleaseResponse{
		Release: &model_helm.Release{
			Name:         "test-test-service",
			Revision:     1,
			Namespace:    "test",
			Status:       "DEPLOYED",
			ChartName:    "test-service",
			ChartVersion: "0.1.0",
		},
	}
	helmInstallRespB, err := json.Marshal(helmInstallResp)
	suite.Nil(err, "no error marshal json")

	resp := &model.Response{
		Key:     cmd.Key,
		Type:    model.HelmInstallRelease,
		Payload: string(helmInstallRespB),
	}

	suite.responseChan <- resp
}

func TestClientSuite(t *testing.T) {
	suite.Run(t, new(ClientTestSuite))
}
