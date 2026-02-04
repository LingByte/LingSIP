package main

import (
	"context"
	"flag"
	"log"
	"os"
	"time"

	"github.com/LingByte/LingSIP/cmd/bootstrap"
	"github.com/LingByte/LingSIP/internal/models"
	"github.com/LingByte/LingSIP/pkg/config"
	"github.com/LingByte/LingSIP/pkg/llm"
	"github.com/LingByte/LingSIP/pkg/logger"
	sip1 "github.com/LingByte/LingSIP/pkg/sip"
	"github.com/LingByte/LingSIP/pkg/sip/ua"
	"github.com/LingByte/LingSIP/pkg/utils"
	"github.com/sirupsen/logrus"
	"go.uber.org/zap"
)

func main() {
	// 1. Parse Command Line Parameters
	mode := flag.String("mode", "", "running environment (development, test, production)")
	init := flag.Bool("init", false, "initialize database")
	initSQL := flag.String("init-sql", "", "path to database init .sql script (optional)")
	flag.Parse()

	// 2. Set Environment Variables
	if *mode != "" {
		os.Setenv("APP_ENV", *mode)
	}
	// 3. Load Global Configuration
	if err := config.Load(); err != nil {
		panic("config load failed: " + err.Error())
	}

	// 4. Load Log Configuration
	err := logger.Init(&config.GlobalConfig.Log, config.GlobalConfig.Server.Mode)
	if err != nil {
		panic(err)
	}

	// 5. Print Banner
	if err := bootstrap.PrintBannerFromFile("banner.txt", config.GlobalConfig.Server.Name); err != nil {
		log.Fatalf("unload banner: %v", err)
	}

	// 7. Load Data Source
	db, err := bootstrap.SetupDatabase(os.Stdout, &bootstrap.Options{
		InitSQLPath: *initSQL, // Can be specified via --init-sql
		AutoMigrate: *init,    // Whether to migrate entities
		SeedNonProd: *init,    // Non-production default configuration
	})
	if err != nil {
		logger.Error("database setup failed", zap.Error(err))
		return
	}

	// 8. Load Base Configs
	var addr = config.GlobalConfig.Server.Addr
	if addr == "" {
		addr = ":7075"
	}

	var DBDriver = config.GlobalConfig.Database.Driver
	if DBDriver == "" {
		DBDriver = "sqlite"
	}

	var DSN = config.GlobalConfig.Database.DSN
	if DSN == "" {
		DSN = "file::memory:?cache=shared"
	}
	flag.StringVar(&addr, "addr", addr, "HTTP Serve address")
	flag.StringVar(&DBDriver, "db-driver", DBDriver, "database driver")
	flag.StringVar(&DSN, "dsn", DSN, "database source name")

	logger.Info("checked config -- addr: ", zap.String("addr", addr))
	logger.Info("checked config -- db-driver: ", zap.String("db-driver", DBDriver), zap.String("dsn", DSN))
	logger.Info("checked config -- mode: ", zap.String("mode", config.GlobalConfig.Server.Mode))

	// 9. Initialize Global Cache
	utils.InitGlobalCache(1024, 5*time.Minute)

	// 10. Initialize LLM Service
	llmConfig := llm.DefaultConfig()
	// 创建一个简单的logrus logger用于LLM服务
	llmLogger := &logrus.Logger{
		Out:       os.Stdout,
		Formatter: &logrus.TextFormatter{},
		Level:     logrus.InfoLevel,
	}
	llmService := llm.NewService(llmConfig, llmLogger)

	// Initialize LLM with system prompt
	systemPrompt := `你是成都市金牛区就业局的工作人员，负责通过电话了解市民的就业需求。

你的任务：
1. 礼貌地询问对方是否有就业相关需求
2. 如果用户只是问候或确认能听到，要继续询问具体的就业需求
3. 如果有需求，了解具体需要什么帮助（找工作、就业培训、创业服务等）
4. 如果明确表示没有需求，礼貌地告知可以前往就近的街道或社区便民服务中心
5. 保持专业、友好的语调

对话策略：
- 如果用户说"你好"、"喂"、"能听到"等问候语，回复后要主动询问就业需求
- 如果用户回答模糊，要进一步澄清
- 每次回复不要超过50个字
- 语言要自然流畅，适合电话对话

注意事项：
- 不要过早结束对话
- 确保了解用户的真实需求后再结束
- 如果用户表示要结束通话，请礼貌地道别`

	ctx := context.Background()
	if err := llmService.Initialize(ctx, systemPrompt); err != nil {
		logger.Warn("LLM service initialization failed, will use mock responses", zap.Error(err))
	} else {
		logger.Info("LLM service initialized successfully")
	}

	server, err := sip1.NewSipServer(10000, 5060, &ua.UAConfig{
		Host:                  "0.0.0.0",
		Port:                  5060,
		UserAgentName:         ua.DEFAULT_USER_AGENT,
		LocalRTPPort:          10000, // 修改为12000避免端口冲突
		RegisterTimeout:       30 * time.Second,
		TransactionTimeout:    30 * time.Second,
		KeepAliveInterval:     60 * time.Second,
		MaxForwards:           70,
		EnableAuthentication:  false,
		AuthenticationRealm:   ua.DEFAULT_REALM_NAME,
		EnableTLS:             false,
		TLSCertFile:           "",
		TLSKeyFile:            "",
		LogLevel:              "info",
		LogFile:               "",
		RTPBufferSize:         1500, // standard 以太网 MTU Size
		MaxConcurrentSessions: 100,
		SessionTimeout:        10 * time.Minute,
		NetworkInterface:      "",
		EnableICE:             false,
		StorageType:           ua.StorageTypeDatabase,
		RegisteredUsers:       make(map[string]string),
		PendingSessions:       make(map[string]string),
		MemoryCalls:           make(map[string]*models.SipCall),
		ActiveSessions:        make(map[string]*ua.SessionInfo),
		Db:                    db,
	})
	if err != nil {
		panic(err)
	}
	defer server.Close()

	// 11. Set LLM Service to AI Phone Engine
	if server.GetAIPhoneEngine() != nil {
		server.GetAIPhoneEngine().SetLLMService(llmService)
		logger.Info("LLM service attached to AI Phone Engine")
	}
	logger.Info("SIP Server Started AT 5060")
	server.Start()

	select {}
}
