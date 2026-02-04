package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/LingByte/LingSIP/pkg/constants"
	"gorm.io/gorm"
)

// SIPTrunkStatus SIP中继状态
type SIPTrunkStatus string

const (
	SIPTrunkStatusActive   SIPTrunkStatus = "active"   // 激活
	SIPTrunkStatusInactive SIPTrunkStatus = "inactive" // 停用
	SIPTrunkStatusError    SIPTrunkStatus = "error"    // 错误
)

// SIPTrunkProvider SIP中继服务商
type SIPTrunkProvider string

const (
	SIPTrunkProviderAliyun  SIPTrunkProvider = "aliyun"  // 阿里云
	SIPTrunkProviderTencent SIPTrunkProvider = "tencent" // 腾讯云
	SIPTrunkProviderTwilio  SIPTrunkProvider = "twilio"  // Twilio
	SIPTrunkProviderCustom  SIPTrunkProvider = "custom"  // 自定义
)

// CodecConfig 编解码器配置
type CodecConfig struct {
	Name     string `json:"name"`     // 编解码器名称 (PCMU, PCMA, G722, etc.)
	Priority int    `json:"priority"` // 优先级
	Enabled  bool   `json:"enabled"`  // 是否启用
}

// CodecConfigs 编解码器配置列表
type CodecConfigs []CodecConfig

// Value 实现 driver.Valuer 接口
func (cc CodecConfigs) Value() (driver.Value, error) {
	if cc == nil || len(cc) == 0 {
		return nil, nil
	}
	return json.Marshal(cc)
}

// Scan 实现 sql.Scanner 接口
func (cc *CodecConfigs) Scan(value interface{}) error {
	if value == nil {
		*cc = make(CodecConfigs, 0)
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	if len(bytes) == 0 {
		*cc = make(CodecConfigs, 0)
		return nil
	}
	return json.Unmarshal(bytes, cc)
}

// PhoneNumbers 电话号码列表
type PhoneNumbers []string

// Value 实现 driver.Valuer 接口
func (pn PhoneNumbers) Value() (driver.Value, error) {
	if pn == nil || len(pn) == 0 {
		return nil, nil
	}
	return json.Marshal(pn)
}

// Scan 实现 sql.Scanner 接口
func (pn *PhoneNumbers) Scan(value interface{}) error {
	if value == nil {
		*pn = make(PhoneNumbers, 0)
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	if len(bytes) == 0 {
		*pn = make(PhoneNumbers, 0)
		return nil
	}
	return json.Unmarshal(bytes, pn)
}

// SIPTrunk SIP中继配置表
type SIPTrunk struct {
	ID        uint       `json:"id" gorm:"primaryKey"`
	CreatedAt time.Time  `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt time.Time  `json:"updatedAt" gorm:"autoUpdateTime"`
	DeletedAt *time.Time `json:"-" gorm:"index"`

	// 基本信息
	Name        string           `json:"name" gorm:"size:128;not null"`                  // 中继名称
	Description string           `json:"description,omitempty" gorm:"type:text"`         // 描述
	Provider    SIPTrunkProvider `json:"provider" gorm:"size:32;not null"`               // 服务商
	Status      SIPTrunkStatus   `json:"status" gorm:"size:20;default:'inactive';index"` // 状态

	// SIP服务器配置
	SIPServer     string `json:"sipServer" gorm:"size:256;not null"`      // SIP服务器地址
	SIPPort       int    `json:"sipPort" gorm:"default:5060"`             // SIP端口
	Domain        string `json:"domain" gorm:"size:128"`                  // SIP域名
	OutboundProxy string `json:"outboundProxy,omitempty" gorm:"size:256"` // 出站代理

	// 认证信息
	Username string `json:"username" gorm:"size:128;not null"`  // 认证用户名
	Password string `json:"-" gorm:"size:256"`                  // 认证密码（加密存储）
	AuthName string `json:"authName,omitempty" gorm:"size:128"` // 认证名称（如果与用户名不同）

	// 号码配置
	PhoneNumbers PhoneNumbers `json:"phoneNumbers" gorm:"type:json"`     // 分配的电话号码列表
	CallerID     string       `json:"callerId,omitempty" gorm:"size:32"` // 默认主叫显示号码

	// 编解码器配置
	Codecs CodecConfigs `json:"codecs" gorm:"type:json"` // 支持的编解码器

	// DTMF配置
	DTMFMode    string `json:"dtmfMode" gorm:"size:20;default:'rfc2833'"` // DTMF模式: rfc2833, inband, info
	DTMFPayload int    `json:"dtmfPayload" gorm:"default:101"`            // DTMF载荷类型

	// 网络配置
	LocalIP    string `json:"localIp,omitempty" gorm:"size:64"`     // 本地IP（多网卡时指定）
	NATEnabled bool   `json:"natEnabled" gorm:"default:false"`      // 是否启用NAT
	STUNServer string `json:"stunServer,omitempty" gorm:"size:256"` // STUN服务器

	// 呼叫配置
	MaxConcurrentCalls int `json:"maxConcurrentCalls" gorm:"default:10"` // 最大并发呼叫数
	CallTimeout        int `json:"callTimeout" gorm:"default:30"`        // 呼叫超时（秒）
	RegisterInterval   int `json:"registerInterval" gorm:"default:3600"` // 注册间隔（秒）

	// 质量配置
	JitterBuffer   int  `json:"jitterBuffer" gorm:"default:50"`     // 抖动缓冲区大小（ms）
	EchoCancel     bool `json:"echoCancel" gorm:"default:true"`     // 回声消除
	NoiseReduction bool `json:"noiseReduction" gorm:"default:true"` // 噪声抑制

	// 统计信息
	TotalCalls   int        `json:"totalCalls" gorm:"default:0"`   // 总呼叫数
	SuccessCalls int        `json:"successCalls" gorm:"default:0"` // 成功呼叫数
	FailedCalls  int        `json:"failedCalls" gorm:"default:0"`  // 失败呼叫数
	LastCallTime *time.Time `json:"lastCallTime,omitempty"`        // 最后呼叫时间
	LastRegister *time.Time `json:"lastRegister,omitempty"`        // 最后注册时间

	// 启用状态
	Enabled   bool `json:"enabled" gorm:"default:true"`    // 是否启用
	IsDefault bool `json:"isDefault" gorm:"default:false"` // 是否为默认中继

	// 元数据
	Metadata string `json:"metadata,omitempty" gorm:"type:text"` // JSON格式的额外配置
	Notes    string `json:"notes,omitempty" gorm:"type:text"`    // 备注
}

// TableName 指定表名
func (SIPTrunk) TableName() string {
	return constants.TABLE_SIP_TRUNKS
}

// IsActive 检查中继是否激活
func (st *SIPTrunk) IsActive() bool {
	return st.Status == SIPTrunkStatusActive && st.Enabled
}

// GetPhoneNumber 获取指定的电话号码，如果不存在则返回第一个
func (st *SIPTrunk) GetPhoneNumber(number string) string {
	if number != "" {
		for _, phone := range st.PhoneNumbers {
			if phone == number {
				return phone
			}
		}
	}
	if len(st.PhoneNumbers) > 0 {
		return st.PhoneNumbers[0]
	}
	return ""
}

// GetDefaultCodecs 获取默认编解码器配置
func GetDefaultCodecs() CodecConfigs {
	return CodecConfigs{
		{Name: "PCMU", Priority: 1, Enabled: true},
		{Name: "PCMA", Priority: 2, Enabled: true},
		{Name: "G722", Priority: 3, Enabled: false},
		{Name: "G729", Priority: 4, Enabled: false},
	}
}

// CRUD 操作函数

// CreateSIPTrunk 创建SIP中继
func CreateSIPTrunk(db *gorm.DB, trunk *SIPTrunk) error {
	// 设置默认编解码器
	if len(trunk.Codecs) == 0 {
		trunk.Codecs = GetDefaultCodecs()
	}
	return db.Create(trunk).Error
}

// GetSIPTrunkByID 根据ID获取SIP中继
func GetSIPTrunkByID(db *gorm.DB, id uint) (*SIPTrunk, error) {
	var trunk SIPTrunk
	err := db.First(&trunk, id).Error
	if err != nil {
		return nil, err
	}
	return &trunk, nil
}

// GetSIPTrunkByName 根据名称获取SIP中继
func GetSIPTrunkByName(db *gorm.DB, name string) (*SIPTrunk, error) {
	var trunk SIPTrunk
	err := db.Where("name = ?", name).First(&trunk).Error
	if err != nil {
		return nil, err
	}
	return &trunk, nil
}

// GetDefaultSIPTrunk 获取默认SIP中继
func GetDefaultSIPTrunk(db *gorm.DB) (*SIPTrunk, error) {
	var trunk SIPTrunk
	err := db.Where("is_default = ? AND enabled = ?", true, true).First(&trunk).Error
	if err != nil {
		return nil, err
	}
	return &trunk, nil
}

// GetActiveSIPTrunks 获取所有激活的SIP中继
func GetActiveSIPTrunks(db *gorm.DB) ([]SIPTrunk, error) {
	var trunks []SIPTrunk
	err := db.Where("status = ? AND enabled = ?", SIPTrunkStatusActive, true).Find(&trunks).Error
	return trunks, err
}

// GetSIPTrunkByPhoneNumber 根据电话号码获取SIP中继
func GetSIPTrunkByPhoneNumber(db *gorm.DB, phoneNumber string) (*SIPTrunk, error) {
	var trunks []SIPTrunk
	err := db.Where("enabled = ?", true).Find(&trunks).Error
	if err != nil {
		return nil, err
	}

	// 遍历查找包含该号码的中继
	for _, trunk := range trunks {
		for _, phone := range trunk.PhoneNumbers {
			if phone == phoneNumber {
				return &trunk, nil
			}
		}
	}

	return nil, gorm.ErrRecordNotFound
}

// UpdateSIPTrunk 更新SIP中继
func UpdateSIPTrunk(db *gorm.DB, trunk *SIPTrunk) error {
	return db.Save(trunk).Error
}

// DeleteSIPTrunk 删除SIP中继（软删除）
func DeleteSIPTrunk(db *gorm.DB, id uint) error {
	return db.Delete(&SIPTrunk{}, id).Error
}

// SetDefaultSIPTrunk 设置默认SIP中继
func SetDefaultSIPTrunk(db *gorm.DB, id uint) error {
	return db.Transaction(func(tx *gorm.DB) error {
		// 取消所有中继的默认状态
		if err := tx.Model(&SIPTrunk{}).Update("is_default", false).Error; err != nil {
			return err
		}
		// 设置指定中继为默认
		return tx.Model(&SIPTrunk{}).Where("id = ?", id).Update("is_default", true).Error
	})
}
