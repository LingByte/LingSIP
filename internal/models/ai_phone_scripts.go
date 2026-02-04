package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/LingByte/LingSIP/pkg/constants"
	"gorm.io/gorm"
)

// ScriptStatus 脚本状态
type ScriptStatus string

const (
	ScriptStatusDraft    ScriptStatus = "draft"    // 草稿
	ScriptStatusActive   ScriptStatus = "active"   // 激活
	ScriptStatusInactive ScriptStatus = "inactive" // 停用
	ScriptStatusArchived ScriptStatus = "archived" // 归档
)

// StepType 步骤类型
type StepType string

const (
	StepTypeCallout   StepType = "callout"   // AI对话
	StepTypePlayAudio StepType = "playaudio" // 播放音频
	StepTypeCollect   StepType = "collect"   // 收集信息
	StepTypeTransfer  StepType = "transfer"  // 转接
	StepTypeHangup    StepType = "hangup"    // 挂断
	StepTypeCondition StepType = "condition" // 条件判断
	StepTypeWait      StepType = "wait"      // 等待
	StepTypeRecord    StepType = "record"    // 录音
	StepTypeDTMF      StepType = "dtmf"      // DTMF按键检测
)

// StepData 步骤数据结构
type StepData struct {
	// AI对话相关
	Prompt    string `json:"prompt,omitempty"`    // AI提示词
	Welcome   string `json:"welcome,omitempty"`   // 开场白
	SpeakerID string `json:"speakerId,omitempty"` // TTS音色ID
	SliceTime int    `json:"sliceTime,omitempty"` // 对话时长限制(ms)
	MaxRetry  int    `json:"maxRetry,omitempty"`  // 最大重试次数

	// 条件判断相关
	Condition string `json:"condition,omitempty"` // 判断条件表达式
	TrueNext  string `json:"trueNext,omitempty"`  // 条件为真时的下一步
	FalseNext string `json:"falseNext,omitempty"` // 条件为假时的下一步

	// 信息收集相关
	CollectKey  string `json:"collectKey,omitempty"`  // 收集的信息键名
	Validation  string `json:"validation,omitempty"`  // 验证规则
	Required    bool   `json:"required,omitempty"`    // 是否必填
	CollectType string `json:"collectType,omitempty"` // 收集类型：text, number, phone, email

	// 音频播放相关
	AudioFile string `json:"audioFile,omitempty"` // 音频文件路径
	AudioText string `json:"audioText,omitempty"` // 音频文本（用于TTS）

	// 转接相关
	TransferTo   string `json:"transferTo,omitempty"`   // 转接目标
	TransferType string `json:"transferType,omitempty"` // 转接类型：human, ivr, external

	// 等待相关
	WaitTime int `json:"waitTime,omitempty"` // 等待时长(ms)

	// 录音相关
	RecordTime   int    `json:"recordTime,omitempty"`   // 录音时长(ms)
	RecordPrompt string `json:"recordPrompt,omitempty"` // 录音提示语

	// DTMF按键相关
	DTMFTimeout    int               `json:"dtmfTimeout,omitempty"`    // DTMF等待超时(ms)
	DTMFMaxDigits  int               `json:"dtmfMaxDigits,omitempty"`  // 最大按键数量
	DTMFTerminator string            `json:"dtmfTerminator,omitempty"` // 结束按键（如#）
	DTMFOptions    map[string]string `json:"dtmfOptions,omitempty"`    // 按键选项映射 {"1": "next_step_id"}
	DTMFPrompt     string            `json:"dtmfPrompt,omitempty"`     // DTMF提示语

	// 通用
	NextStep  string                 `json:"nextStep,omitempty"`  // 下一步骤ID
	Variables map[string]string      `json:"variables,omitempty"` // 变量设置
	Metadata  map[string]interface{} `json:"metadata,omitempty"`  // 元数据
}

// Value 实现 driver.Valuer 接口
func (sd StepData) Value() (driver.Value, error) {
	return json.Marshal(sd)
}

// Scan 实现 sql.Scanner 接口
func (sd *StepData) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, sd)
}

// AIPhoneScript AI电话脚本表
type AIPhoneScript struct {
	ID        uint       `json:"id" gorm:"primaryKey"`
	CreatedAt time.Time  `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt time.Time  `json:"updatedAt" gorm:"autoUpdateTime"`
	DeletedAt *time.Time `json:"-" gorm:"index"`

	// 基本信息
	Name        string       `json:"name" gorm:"size:128;not null;index"`         // 脚本名称
	Description string       `json:"description,omitempty" gorm:"type:text"`      // 脚本描述
	Version     string       `json:"version" gorm:"size:32;default:'1.0.0'"`      // 版本号
	Status      ScriptStatus `json:"status" gorm:"size:20;default:'draft';index"` // 状态

	// 脚本配置
	SpeakerID   string `json:"speakerId" gorm:"size:32"`                // 默认说话人ID
	StartStepID string `json:"startStepId" gorm:"size:64"`              // 起始步骤ID
	Category    string `json:"category,omitempty" gorm:"size:64;index"` // 分类
	Tags        string `json:"tags,omitempty" gorm:"size:256"`          // 标签（逗号分隔）

	// 业务配置
	BusinessType string `json:"businessType,omitempty" gorm:"size:64"` // 业务类型
	Department   string `json:"department,omitempty" gorm:"size:128"`  // 所属部门
	Owner        string `json:"owner,omitempty" gorm:"size:64"`        // 负责人

	// 执行配置
	MaxDuration   int    `json:"maxDuration" gorm:"default:300000"`      // 最大通话时长(ms)
	MaxSteps      int    `json:"maxSteps" gorm:"default:50"`             // 最大步骤数
	TimeoutAction string `json:"timeoutAction,omitempty" gorm:"size:32"` // 超时动作

	// 统计信息
	ExecuteCount int        `json:"executeCount" gorm:"default:0"` // 执行次数
	SuccessCount int        `json:"successCount" gorm:"default:0"` // 成功次数
	LastExecute  *time.Time `json:"lastExecute,omitempty"`         // 最后执行时间

	// 关联关系
	Steps         []AIPhoneScriptStep  `json:"steps,omitempty" gorm:"foreignKey:ScriptID;constraint:OnDelete:CASCADE"`
	PhoneMappings []ScriptPhoneMapping `json:"phoneMappings,omitempty" gorm:"foreignKey:ScriptID;constraint:OnDelete:CASCADE"`
}

// TableName 指定表名
func (AIPhoneScript) TableName() string {
	return constants.TABLE_AI_PHONE_SCRIPTS
}

// AIPhoneScriptStep AI电话脚本步骤表
type AIPhoneScriptStep struct {
	ID        uint       `json:"id" gorm:"primaryKey"`
	CreatedAt time.Time  `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt time.Time  `json:"updatedAt" gorm:"autoUpdateTime"`
	DeletedAt *time.Time `json:"-" gorm:"index"`

	// 关联信息
	ScriptID uint   `json:"scriptId" gorm:"not null;index"`         // 脚本ID
	StepID   string `json:"stepId" gorm:"size:64;not null;index"`   // 步骤ID（业务ID）
	GroupID  string `json:"groupId,omitempty" gorm:"size:64;index"` // 步骤组ID

	// 步骤信息
	Name  string   `json:"name" gorm:"size:128;not null"`      // 步骤名称
	Type  StepType `json:"type" gorm:"size:20;not null;index"` // 步骤类型
	Order int      `json:"order" gorm:"default:0;index"`       // 排序

	// 步骤数据
	Data StepData `json:"data" gorm:"type:json"` // 步骤配置数据

	// 执行配置
	Enabled bool `json:"enabled" gorm:"default:true"`  // 是否启用
	Timeout int  `json:"timeout" gorm:"default:30000"` // 超时时间(ms)

	// 统计信息
	ExecuteCount int `json:"executeCount" gorm:"default:0"` // 执行次数
	SuccessCount int `json:"successCount" gorm:"default:0"` // 成功次数
	ErrorCount   int `json:"errorCount" gorm:"default:0"`   // 错误次数

	// 关联关系
	Script AIPhoneScript `json:"script,omitempty" gorm:"foreignKey:ScriptID"`
}

// TableName 指定表名
func (AIPhoneScriptStep) TableName() string {
	return constants.TABLE_AI_PHONE_SCRIPT_STEPS
}

// ScriptPhoneMapping 脚本电话号码映射表
type ScriptPhoneMapping struct {
	ID        uint       `json:"id" gorm:"primaryKey"`
	CreatedAt time.Time  `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt time.Time  `json:"updatedAt" gorm:"autoUpdateTime"`
	DeletedAt *time.Time `json:"-" gorm:"index"`

	// 映射信息
	ScriptID    uint   `json:"scriptId" gorm:"not null;index"`                  // 脚本ID
	PhoneNumber string `json:"phoneNumber" gorm:"size:20;not null;uniqueIndex"` // 电话号码
	Priority    int    `json:"priority" gorm:"default:0"`                       // 优先级

	// 配置信息
	Enabled     bool   `json:"enabled" gorm:"default:true"`           // 是否启用
	Description string `json:"description,omitempty" gorm:"size:256"` // 描述

	// 时间限制
	StartTime string `json:"startTime,omitempty" gorm:"size:8"` // 开始时间 HH:MM:SS
	EndTime   string `json:"endTime,omitempty" gorm:"size:8"`   // 结束时间 HH:MM:SS
	WeekDays  string `json:"weekDays,omitempty" gorm:"size:16"` // 工作日 1,2,3,4,5,6,7

	// 统计信息
	CallCount int        `json:"callCount" gorm:"default:0"` // 呼叫次数
	LastCall  *time.Time `json:"lastCall,omitempty"`         // 最后呼叫时间

	// 关联关系
	Script AIPhoneScript `json:"script,omitempty" gorm:"foreignKey:ScriptID"`
}

// TableName 指定表名
func (ScriptPhoneMapping) TableName() string {
	return constants.TABLE_SCRIPT_PHONE_MAPPINGS
}

// CRUD 操作函数

// CreateAIPhoneScript 创建AI电话脚本
func CreateAIPhoneScript(db *gorm.DB, script *AIPhoneScript) error {
	return db.Create(script).Error
}

// GetAIPhoneScriptByID 根据ID获取脚本
func GetAIPhoneScriptByID(db *gorm.DB, id uint) (*AIPhoneScript, error) {
	var script AIPhoneScript
	err := db.Preload("Steps").Preload("PhoneMappings").First(&script, id).Error
	if err != nil {
		return nil, err
	}
	return &script, nil
}

// GetAIPhoneScriptByName 根据名称获取脚本
func GetAIPhoneScriptByName(db *gorm.DB, name string) (*AIPhoneScript, error) {
	var script AIPhoneScript
	err := db.Preload("Steps").Preload("PhoneMappings").Where("name = ? AND status != ?", name, ScriptStatusArchived).First(&script).Error
	if err != nil {
		return nil, err
	}
	return &script, nil
}

// GetAIPhoneScriptByPhone 根据电话号码获取脚本
func GetAIPhoneScriptByPhone(db *gorm.DB, phoneNumber string) (*AIPhoneScript, error) {
	var mapping ScriptPhoneMapping
	err := db.Preload("Script").Preload("Script.Steps").Where("phone_number = ? AND enabled = ?", phoneNumber, true).First(&mapping).Error
	if err != nil {
		return nil, err
	}
	return &mapping.Script, nil
}

// GetActiveAIPhoneScripts 获取所有激活的脚本
func GetActiveAIPhoneScripts(db *gorm.DB) ([]AIPhoneScript, error) {
	var scripts []AIPhoneScript
	err := db.Where("status = ?", ScriptStatusActive).Find(&scripts).Error
	return scripts, err
}

// UpdateAIPhoneScript 更新脚本
func UpdateAIPhoneScript(db *gorm.DB, script *AIPhoneScript) error {
	return db.Save(script).Error
}

// DeleteAIPhoneScript 删除脚本（软删除）
func DeleteAIPhoneScript(db *gorm.DB, id uint) error {
	return db.Delete(&AIPhoneScript{}, id).Error
}

// CreateScriptStep 创建脚本步骤
func CreateScriptStep(db *gorm.DB, step *AIPhoneScriptStep) error {
	return db.Create(step).Error
}

// GetScriptStepsByScriptID 获取脚本的所有步骤
func GetScriptStepsByScriptID(db *gorm.DB, scriptID uint) ([]AIPhoneScriptStep, error) {
	var steps []AIPhoneScriptStep
	err := db.Where("script_id = ?", scriptID).Order("`order` ASC, created_at ASC").Find(&steps).Error
	return steps, err
}

// GetScriptStepByStepID 根据步骤ID获取步骤
func GetScriptStepByStepID(db *gorm.DB, scriptID uint, stepID string) (*AIPhoneScriptStep, error) {
	var step AIPhoneScriptStep
	err := db.Where("script_id = ? AND step_id = ?", scriptID, stepID).First(&step).Error
	if err != nil {
		return nil, err
	}
	return &step, nil
}

// UpdateScriptStep 更新脚本步骤
func UpdateScriptStep(db *gorm.DB, step *AIPhoneScriptStep) error {
	return db.Save(step).Error
}

// DeleteScriptStep 删除脚本步骤
func DeleteScriptStep(db *gorm.DB, id uint) error {
	return db.Delete(&AIPhoneScriptStep{}, id).Error
}

// CreateScriptPhoneMapping 创建脚本电话映射
func CreateScriptPhoneMapping(db *gorm.DB, mapping *ScriptPhoneMapping) error {
	return db.Create(mapping).Error
}

// UpdateScriptPhoneMapping 更新脚本电话映射
func UpdateScriptPhoneMapping(db *gorm.DB, mapping *ScriptPhoneMapping) error {
	return db.Save(mapping).Error
}

// DeleteScriptPhoneMapping 删除脚本电话映射
func DeleteScriptPhoneMapping(db *gorm.DB, id uint) error {
	return db.Delete(&ScriptPhoneMapping{}, id).Error
}

// 辅助方法

// IsActive 检查脚本是否激活
func (s *AIPhoneScript) IsActive() bool {
	return s.Status == ScriptStatusActive
}

// GetStepByID 获取指定ID的步骤
func (s *AIPhoneScript) GetStepByID(stepID string) *AIPhoneScriptStep {
	for _, step := range s.Steps {
		if step.StepID == stepID {
			return &step
		}
	}
	return nil
}

// GetStartStep 获取起始步骤
func (s *AIPhoneScript) GetStartStep() *AIPhoneScriptStep {
	return s.GetStepByID(s.StartStepID)
}

// IncrementExecuteCount 增加执行次数
func (s *AIPhoneScript) IncrementExecuteCount(db *gorm.DB) error {
	now := time.Now()
	return db.Model(s).Updates(map[string]interface{}{
		"execute_count": gorm.Expr("execute_count + 1"),
		"last_execute":  now,
	}).Error
}

// IncrementSuccessCount 增加成功次数
func (s *AIPhoneScript) IncrementSuccessCount(db *gorm.DB) error {
	return db.Model(s).Update("success_count", gorm.Expr("success_count + 1")).Error
}
