package websocket

import (
	"encoding/json"
	"fmt"
	"github.com/choerodon/choerodon-cluster-agent/manager"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"

	model_helm "github.com/choerodon/choerodon-cluster-agent/pkg/helm"
	"github.com/choerodon/choerodon-cluster-agent/pkg/model"
)

var (
	clientTestUpgrader = websocket.Upgrader{}
)

func clientTestRouter(t *testing.T) http.Handler {
	router := gin.Default()
	router.GET("/agent", func(c *gin.Context) {
		clientTestServeWs(t, c.Writer, c.Request)
	})
	return router
}

func clientTestServeWs(t *testing.T, w http.ResponseWriter, r *http.Request) {
	conn, err := clientTestUpgrader.Upgrade(w, r, nil)
	assert.Nil(t, err, "no error upgrades")
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
	command := &model.Packet{
		Key:     "helm:release:install",
		Type:    model.HelmInstallRelease,
		Payload: string(commandMsgPayloadBytes),
	}

	err = conn.WriteJSON(command)
	assert.Nil(t, err, "no error write json")

	_, _, err = conn.ReadMessage()
	assert.Nil(t, err, "no error read message")

	conn.WriteMessage(websocket.CloseMessage, []byte{})
}

func TestClient(t *testing.T) {
	commandChan := make(chan *model.Packet)
	responseChan := make(chan *model.Packet)
	shutdown := make(chan struct{})
	shutdownWg := &sync.WaitGroup{}
	server := httptest.NewServer(clientTestRouter(t))
	defer server.Close()

	crChan := &manager.CRChan{
		ResponseChan: responseChan,
		CommandChan:  commandChan,
	}

	serverURL, _ := url.Parse(fmt.Sprintf("ws%s/agent", strings.TrimPrefix(server.URL, "http")))
	c, err := NewClient(Token("token"), serverURL.String(), crChan)
	assert.Nil(t, err, "no error create new client")

	go c.Loop(shutdown, shutdownWg)

	cmd := <-commandChan
	assert.Equal(t, "helm:release:install", cmd.Key, "Bad command")

	helmInstallResp := &model_helm.Release{
		Name:         "test-test-service",
		Revision:     1,
		Namespace:    "test",
		Status:       "DEPLOYED",
		ChartName:    "test-service",
		ChartVersion: "0.1.0",
	}
	helmInstallRespB, err := json.Marshal(helmInstallResp)
	assert.Nil(t, err, "no error marshal json")

	resp := &model.Packet{
		Key:     cmd.Key,
		Type:    model.HelmInstallRelease,
		Payload: string(helmInstallRespB),
	}

	responseChan <- resp
}
