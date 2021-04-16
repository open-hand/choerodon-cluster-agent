package model

import (
	"sync"
)

var CertManagerVersion string

var KubernetesVersion string

var ClusterId string

var AgentVersion string

var AgentNamespace string

var MaxWebsocketMessageLength int

var MaxJobLogLength int

// agent是否已初始化的标志。 如果websocket连接断开,将此置为false。如果agent.InitAgent执行成功，将此置为true
var Initialized = false

// websocket重连标志
var ReconnectFlag = false

// gitops监听协程退出通道
var GitStopChanMap = make(map[string]chan struct{})

// gitops监听协程是否在运行中标志
var GitRunning = false

// agent初始化锁，防止接收多个agent_init命令，导致重复初始化
var InitLock = sync.Mutex{}

// git repo初始化的时候控制并发的chan
var GitRepoConcurrencySyncChan chan struct{}
