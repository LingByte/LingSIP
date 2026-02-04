package sip1

import (
	"fmt"

	"github.com/LingByte/LingSIP/internal/models"
	"gorm.io/gorm"
)

// RongLianTrunkConfig 容联云通讯SIP中继配置
type RongLianTrunkConfig struct {
	Name         string   `json:"name"`         // 中继名称
	AccountSID   string   `json:"accountSid"`   // 账户SID
	AuthToken    string   `json:"authToken"`    // 认证令牌
	AppID        string   `json:"appId"`        // 应用ID
	AppToken     string   `json:"appToken"`     // 应用令牌
	PhoneNumbers []string `json:"phoneNumbers"` // 电话号码列表
	CallerID     string   `json:"callerId"`     // 主叫显示号码
}

// CreateRongLianTrunk 创建容联云SIP中继配置
func CreateRongLianTrunk(db *gorm.DB, config *RongLianTrunkConfig) (*models.SIPTrunk, error) {
	// 验证配置
	if config.AccountSID == "" {
		return nil, fmt.Errorf("Account SID is required")
	}
	if config.AuthToken == "" {
		return nil, fmt.Errorf("Auth Token is required")
	}
	if len(config.PhoneNumbers) == 0 {
		return nil, fmt.Errorf("at least one phone number is required")
	}

	// 设置默认值
	if config.Name == "" {
		config.Name = fmt.Sprintf("RongLian-%s", config.AccountSID[:8])
	}

	// 创建SIP中继记录
	trunk := &models.SIPTrunk{
		Name:        config.Name,
		Description: "容联云通讯SIP中继",
		Provider:    models.SIPTrunkProviderCustom,
		Status:      models.SIPTrunkStatusInactive, // 初始状态为未激活

		// SIP服务器配置
		SIPServer: "app.cloopen.com",
		SIPPort:   5060,
		Domain:    "cloopen.com",

		// 认证信息 - 使用Account SID和Auth Token
		Username: config.AccountSID,
		Password: config.AuthToken,

		// 号码配置
		PhoneNumbers: models.PhoneNumbers(config.PhoneNumbers),
		CallerID:     config.CallerID,

		// 编解码器配置（容联云推荐配置）
		Codecs: models.CodecConfigs{
			{Name: "PCMU", Priority: 1, Enabled: true},
			{Name: "PCMA", Priority: 2, Enabled: true},
			{Name: "G729", Priority: 3, Enabled: false},
		},

		// DTMF配置
		DTMFMode:    "rfc2833",
		DTMFPayload: 101,

		// 呼叫配置
		MaxConcurrentCalls: 5, // 测试账户通常限制较低
		CallTimeout:        30,
		RegisterInterval:   3600,

		// 质量配置
		JitterBuffer:   50,
		EchoCancel:     true,
		NoiseReduction: true,

		// 启用状态
		Enabled:   true,
		IsDefault: false,

		// 存储额外信息到Metadata
		Metadata: fmt.Sprintf(`{"appId":"%s","appToken":"%s"}`, config.AppID, config.AppToken),
	}

	// 保存到数据库
	if err := models.CreateSIPTrunk(db, trunk); err != nil {
		return nil, fmt.Errorf("failed to create SIP trunk: %w", err)
	}

	return trunk, nil
}

// GetRongLianTrunkTemplate 获取容联云SIP中继配置模板
func GetRongLianTrunkTemplate() *RongLianTrunkConfig {
	return &RongLianTrunkConfig{
		Name:       "容联云通讯",
		AccountSID: "your_account_sid", // 需要替换
		AuthToken:  "your_auth_token",  // 需要替换
		AppID:      "your_app_id",      // 需要替换
		AppToken:   "your_app_token",   // 需要替换
		PhoneNumbers: []string{
			"400-xxx-xxxx", // 需要替换为实际号码
		},
		CallerID: "400-xxx-xxxx", // 需要替换
	}
}

// ValidateRongLianConfig 验证容联云配置
func ValidateRongLianConfig(config *RongLianTrunkConfig) error {
	if config.AccountSID == "" {
		return fmt.Errorf("Account SID不能为空")
	}

	if config.AuthToken == "" {
		return fmt.Errorf("Auth Token不能为空")
	}

	if len(config.PhoneNumbers) == 0 {
		return fmt.Errorf("至少需要一个电话号码")
	}

	// 验证Account SID格式（通常以AC开头）
	if len(config.AccountSID) < 10 {
		return fmt.Errorf("Account SID格式不正确")
	}

	// 验证电话号码格式
	for _, phone := range config.PhoneNumbers {
		if len(phone) < 7 {
			return fmt.Errorf("电话号码格式不正确: %s", phone)
		}
	}

	return nil
}

// TestRongLianConnection 测试容联云SIP连接
func TestRongLianConnection(config *RongLianTrunkConfig) error {
	// 验证配置
	if err := ValidateRongLianConfig(config); err != nil {
		return err
	}

	// TODO: 实现实际的连接测试
	// 1. 创建临时SIP客户端
	// 2. 尝试注册到容联云SIP服务器
	// 3. 检查响应状态

	return nil
}

// RongLianAPIConfig 容联云API配置（用于非SIP调用）
type RongLianAPIConfig struct {
	BaseURL    string `json:"baseUrl"`    // API基础URL
	AppID      string `json:"appId"`      // 应用ID
	AppToken   string `json:"appToken"`   // 应用令牌
	AccountSID string `json:"accountSid"` // 账户SID
	AuthToken  string `json:"authToken"`  // 认证令牌
}

// GetRongLianAPIConfig 从SIP中继配置获取API配置
func GetRongLianAPIConfig(trunk *models.SIPTrunk) (*RongLianAPIConfig, error) {
	if trunk.Provider != models.SIPTrunkProviderCustom {
		return nil, fmt.Errorf("not a RongLian trunk")
	}

	// 从Metadata中解析AppID和AppToken
	// 这里需要实现JSON解析逻辑

	return &RongLianAPIConfig{
		BaseURL:    "https://app.cloopen.com:8883",
		AccountSID: trunk.Username,
		AuthToken:  trunk.Password,
		// AppID和AppToken需要从Metadata中解析
	}, nil
}

// 容联云特殊功能

// RongLianCallRecord 容联云通话记录
type RongLianCallRecord struct {
	CallSid    string `json:"callSid"`    // 通话SID
	From       string `json:"from"`       // 主叫号码
	To         string `json:"to"`         // 被叫号码
	StartTime  string `json:"startTime"`  // 开始时间
	EndTime    string `json:"endTime"`    // 结束时间
	Duration   int    `json:"duration"`   // 通话时长（秒）
	Status     string `json:"status"`     // 通话状态
	RecordFile string `json:"recordFile"` // 录音文件URL
}

// GetCallRecords 获取容联云通话记录
func GetCallRecords(config *RongLianAPIConfig, date string) ([]RongLianCallRecord, error) {
	// TODO: 实现API调用获取通话记录
	// 1. 构建API请求
	// 2. 发送HTTP请求到容联云API
	// 3. 解析响应数据

	return nil, fmt.Errorf("not implemented")
}

// SendVoiceNotification 发送语音通知（非实时通话）
func SendVoiceNotification(config *RongLianAPIConfig, to, content string) error {
	// TODO: 实现语音通知发送
	// 1. 构建API请求
	// 2. 发送HTTP请求到容联云API
	// 3. 处理响应结果

	return fmt.Errorf("not implemented")
}
