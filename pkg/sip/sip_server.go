package sip1

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/LingByte/LingSIP/pkg/logger"
	"github.com/LingByte/LingSIP/pkg/sip/ua"
	"github.com/emiago/sipgo"
	"go.uber.org/zap"
)

type SipServer struct {
	config  *ua.UAConfig
	ua      *sipgo.UserAgent
	client  *sipgo.Client
	server  *sipgo.Server
	rtpConn *net.UDPConn
	mutex   sync.RWMutex
	running bool

	// AI电话引擎
	aiEngine *AIPhoneEngine
}

func NewSipServer(rptPort, sipPort int, uaConfig *ua.UAConfig) (*SipServer, error) {
	if uaConfig != nil {
		if err := uaConfig.Validate(); err != nil {
			return nil, fmt.Errorf("invalid UA config: %w", err)
		}
	} else {
		uaConfig = ua.DefaultUAConfig()
	}
	uaConfig.ApplyDefaults()

	uaConfig.LocalRTPPort = rptPort
	uaConfig.Port = sipPort

	userAgent, err := sipgo.NewUA(sipgo.WithUserAgent(uaConfig.UserAgentName))
	if err != nil {
		logger.Fatal("Failed to create UA", zap.Error(err))
	}

	server, err := sipgo.NewServer(userAgent)
	if err != nil {
		logger.Fatal("Failed to create SIP server", zap.Error(err))
	}

	// Create RTP UDP connection
	rtpAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("0.0.0.0:%d", rptPort))
	if err != nil {
		logger.Fatal("Failed to resolve RTP address", zap.Error(err))
	}

	rtpConn, err := net.ListenUDP("udp", rtpAddr)
	if err != nil {
		logger.Fatal("Failed to create RTP UDP connection", zap.Error(err))
	}

	client, err := sipgo.NewClient(userAgent)
	if err != nil {
		logger.Fatal("Create SIP Client Failed", zap.Error(err))
	}

	sipServer := &SipServer{
		config:  uaConfig,
		server:  server,
		rtpConn: rtpConn,
		client:  client,
		ua:      userAgent,
	}

	// 初始化AI电话引擎
	if uaConfig.Db != nil {
		sipServer.aiEngine = NewAIPhoneEngine(sipServer, uaConfig.Db)
		logger.Info("AI phone engine initialized")
	}

	return sipServer, nil
}

func (as *SipServer) Close() {
	as.server.Close()
	as.rtpConn.Close()
	as.client.Close()
	as.ua.Close()
	as.running = false
	logger.Info("SIP Server Closed")
}

func (as *SipServer) Start() {
	as.RegisterFunc()

	ctx := context.Background()
	if err := as.server.ListenAndServe(ctx, "udp", fmt.Sprintf("%s:%d", as.config.Host, as.config.Port)); err != nil {
		logger.Fatal("Failed to start server", zap.Error(err))
	}
}
