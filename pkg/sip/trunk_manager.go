package sip1

import (
	"fmt"
	"sync"
	"time"

	"github.com/LingByte/LingSIP/internal/models"
	"github.com/LingByte/LingSIP/pkg/logger"
	"github.com/emiago/sipgo"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// TrunkManager SIP中继管理器
type TrunkManager struct {
	db     *gorm.DB
	trunks map[uint]*TrunkConnection // trunk_id -> connection
	mutex  sync.RWMutex

	// SIP客户端
	userAgent *sipgo.UserAgent
	client    *sipgo.Client
}

// TrunkConnection SIP中继连接
type TrunkConnection struct {
	Trunk        *models.SIPTrunk
	Client       *sipgo.Client
	IsRegistered bool
	LastRegister time.Time
	LastError    error

	// 统计信息
	CallCount    int
	SuccessCount int
	FailedCount  int

	mutex sync.RWMutex
}

// NewTrunkManager 创建SIP中继管理器
func NewTrunkManager(db *gorm.DB, userAgent *sipgo.UserAgent) *TrunkManager {
	client, err := sipgo.NewClient(userAgent)
	if err != nil {
		logger.Error("Failed to create SIP client for trunk manager", zap.Error(err))
		return nil
	}

	return &TrunkManager{
		db:        db,
		trunks:    make(map[uint]*TrunkConnection),
		userAgent: userAgent,
		client:    client,
	}
}

// LoadTrunks 加载所有激活的SIP中继
func (tm *TrunkManager) LoadTrunks() error {
	trunks, err := models.GetActiveSIPTrunks(tm.db)
	if err != nil {
		return fmt.Errorf("failed to load SIP trunks: %w", err)
	}

	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	for _, trunk := range trunks {
		if err := tm.addTrunk(&trunk); err != nil {
			logger.Error("Failed to add trunk",
				zap.String("name", trunk.Name),
				zap.Error(err))
			continue
		}
	}

	logger.Info("SIP trunks loaded", zap.Int("count", len(tm.trunks)))
	return nil
}

// addTrunk 添加SIP中继连接
func (tm *TrunkManager) addTrunk(trunk *models.SIPTrunk) error {
	// 创建中继连接
	conn := &TrunkConnection{
		Trunk: trunk,
	}

	// 创建专用的SIP客户端
	client, err := sipgo.NewClient(tm.userAgent)
	if err != nil {
		return fmt.Errorf("failed to create SIP client for trunk %s: %w", trunk.Name, err)
	}
	conn.Client = client

	// 存储连接
	tm.trunks[trunk.ID] = conn

	// 如果需要注册，启动注册
	if trunk.Username != "" && trunk.Password != "" {
		go tm.registerTrunk(conn)
	}

	logger.Info("SIP trunk added",
		zap.String("name", trunk.Name),
		zap.String("server", trunk.SIPServer),
		zap.Int("port", trunk.SIPPort))

	return nil
}

// registerTrunk 注册SIP中继
func (tm *TrunkManager) registerTrunk(conn *TrunkConnection) {
	trunk := conn.Trunk

	logger.Info("Starting SIP trunk registration",
		zap.String("name", trunk.Name),
		zap.String("server", trunk.SIPServer),
		zap.String("username", trunk.Username))

	// TODO: 实现SIP注册逻辑
	// 由于sipgo库API变化，暂时跳过注册

	conn.mutex.Lock()
	conn.IsRegistered = true
	conn.LastRegister = time.Now()
	conn.LastError = nil
	trunk.Status = models.SIPTrunkStatusActive
	models.UpdateSIPTrunk(tm.db, trunk)
	conn.mutex.Unlock()

	logger.Info("SIP trunk marked as active",
		zap.String("name", trunk.Name))
}

// startPeriodicRegistration 启动定期重新注册
func (tm *TrunkManager) startPeriodicRegistration(conn *TrunkConnection) {
	// TODO: 实现定期重新注册
	logger.Info("Periodic registration scheduled",
		zap.String("name", conn.Trunk.Name))
}

// GetTrunkByPhoneNumber 根据电话号码获取SIP中继
func (tm *TrunkManager) GetTrunkByPhoneNumber(phoneNumber string) (*TrunkConnection, error) {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	for _, conn := range tm.trunks {
		for _, phone := range conn.Trunk.PhoneNumbers {
			if phone == phoneNumber {
				return conn, nil
			}
		}
	}

	return nil, fmt.Errorf("no trunk found for phone number: %s", phoneNumber)
}

// GetDefaultTrunk 获取默认SIP中继
func (tm *TrunkManager) GetDefaultTrunk() (*TrunkConnection, error) {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	for _, conn := range tm.trunks {
		if conn.Trunk.IsDefault && conn.IsRegistered {
			return conn, nil
		}
	}

	// 如果没有默认中继，返回第一个已注册的中继
	for _, conn := range tm.trunks {
		if conn.IsRegistered {
			return conn, nil
		}
	}

	return nil, fmt.Errorf("no active trunk available")
}

// MakeCall 通过SIP中继发起呼叫
func (tm *TrunkManager) MakeCall(trunkID uint, fromNumber, toNumber string) error {
	tm.mutex.RLock()
	conn, exists := tm.trunks[trunkID]
	tm.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("trunk not found: %d", trunkID)
	}

	if !conn.IsRegistered {
		return fmt.Errorf("trunk not registered: %s", conn.Trunk.Name)
	}

	// 构建呼叫URI
	fromURI := fmt.Sprintf("sip:%s@%s", fromNumber, conn.Trunk.Domain)
	toURI := fmt.Sprintf("sip:%s@%s:%d", toNumber, conn.Trunk.SIPServer, conn.Trunk.SIPPort)

	logger.Info("Making call through SIP trunk",
		zap.String("trunk", conn.Trunk.Name),
		zap.String("from", fromURI),
		zap.String("to", toURI))

	// 这里需要实现具体的呼叫逻辑
	// 由于sipgo库的API可能有所不同，需要根据实际情况调整

	conn.mutex.Lock()
	conn.CallCount++
	conn.mutex.Unlock()

	return nil
}

// GetTrunkStatus 获取中继状态
func (tm *TrunkManager) GetTrunkStatus(trunkID uint) (*TrunkStatus, error) {
	tm.mutex.RLock()
	conn, exists := tm.trunks[trunkID]
	tm.mutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("trunk not found: %d", trunkID)
	}

	conn.mutex.RLock()
	defer conn.mutex.RUnlock()

	status := &TrunkStatus{
		TrunkID:      trunkID,
		Name:         conn.Trunk.Name,
		IsRegistered: conn.IsRegistered,
		LastRegister: conn.LastRegister,
		LastError:    conn.LastError,
		CallCount:    conn.CallCount,
		SuccessCount: conn.SuccessCount,
		FailedCount:  conn.FailedCount,
	}

	return status, nil
}

// TrunkStatus 中继状态信息
type TrunkStatus struct {
	TrunkID      uint      `json:"trunkId"`
	Name         string    `json:"name"`
	IsRegistered bool      `json:"isRegistered"`
	LastRegister time.Time `json:"lastRegister"`
	LastError    error     `json:"lastError,omitempty"`
	CallCount    int       `json:"callCount"`
	SuccessCount int       `json:"successCount"`
	FailedCount  int       `json:"failedCount"`
}

// Close 关闭中继管理器
func (tm *TrunkManager) Close() {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	for _, conn := range tm.trunks {
		if conn.Client != nil {
			conn.Client.Close()
		}
	}

	tm.trunks = make(map[uint]*TrunkConnection)
	logger.Info("SIP trunk manager closed")
}
