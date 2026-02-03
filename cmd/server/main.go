package main

import (
	"flag"
	"log"
	"os"
	"time"

	"github.com/LingByte/LingSIP/cmd/bootstrap"
	"github.com/LingByte/LingSIP/internal/models"
	"github.com/LingByte/LingSIP/pkg/config"
	"github.com/LingByte/LingSIP/pkg/logger"
	sip1 "github.com/LingByte/LingSIP/pkg/sip"
	"github.com/LingByte/LingSIP/pkg/sip/ua"
	"github.com/LingByte/LingSIP/pkg/utils"
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

	server, err := sip1.NewSipServer(10000, 5060, &ua.UAConfig{
		Host:                  "0.0.0.0",
		Port:                  5060,
		UserAgentName:         ua.DEFAULT_USER_AGENT,
		LocalRTPPort:          10000, // Default RPT Port
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
	server.Start()
}
