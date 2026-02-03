package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/LingByte/LingSIP/pkg/constants"
	"gorm.io/gorm"
)

// SessionStatus 会话状态
type SessionStatus string

const (
	SessionStatusStarting    SessionStatus = "starting"    // 启动中
	SessionStatusRunning     SessionStatus = "running"     // 运行中
	SessionStatusCompleted   SessionStatus = "completed"   // 已完成
	SessionStatusFailed      SessionStatus = "failed"      // 失败
	SessionStatusTimeout     SessionStatus = "timeout"     // 超时
	SessionStatusCancelled   SessionStatus = "cancelled"   // 已取消
	SessionStatusTransferred SessionStatus = "transferred" // 已转接
)

// StepExecutionStatus 步骤执行状态
type StepExecutionStatus string

const (
	StepStatusPending   StepExecutionStatus = "pending"   // 等待执行
	StepStatusRunning   StepExecutionStatus = "running"   // 执行中
	StepStatusCompleted StepExecutionStatus = "completed" // 已完成
	StepStatusFailed    StepExecutionStatus = "failed"    // 失败
	StepStatusSkipped   StepExecutionStatus = "skipped"   // 跳过
	StepStatusTimeout   StepExecutionStatus = "timeout"   // 超时
)

// SessionContext 会话上下文数据
type SessionContext map[string]interface{}

// Value 实现 driver.Valuer 接口
func (sc SessionContext) Value() (driver.Value, error) {
	if sc == nil || len(sc) == 0 {
		return nil, nil
	}
	return json.Marshal(sc)
}

// Scan 实现 sql.Scanner 接口
func (sc *SessionContext) Scan(value interface{}) error {
	if value == nil {
		*sc = make(SessionContext)
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		*sc = make(SessionContext)
		return nil
	}
	if len(bytes) == 0 {
		*sc = make(SessionContext)
		return nil
	}
	return json.Unmarshal(bytes, sc)
}

// ConversationHistory 对话历史
type ConversationHistory []ConversationMessage

// ConversationMessage 对话消息
type ConversationMessage struct {
	Role      string                 `json:"role"`               // "user" or "assistant"
	Content   string                 `json:"content"`            // 消息内容
	Timestamp time.Time              `json:"timestamp"`          // 时间戳
	StepID    string                 `json:"stepId,omitempty"`   // 关联的步骤ID
	Metadata  map[string]interface{} `json:"metadata,omitempty"` // 元数据
}

// Value 实现 driver.Valuer 接口
func (ch ConversationHistory) Value() (driver.Value, error) {
	if ch == nil || len(ch) == 0 {
		return nil, nil
	}
	return json.Marshal(ch)
}

// Scan 实现 sql.Scanner 接口
func (ch *ConversationHistory) Scan(value interface{}) error {
	if value == nil {
		*ch = make(ConversationHistory, 0)
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		*ch = make(ConversationHistory, 0)
		return nil
	}
	if len(bytes) == 0 {
		*ch = make(ConversationHistory, 0)
		return nil
	}
	return json.Unmarshal(bytes, ch)
}

// AIPhoneSession AI电话会话表
type AIPhoneSession struct {
	ID        uint       `json:"id" gorm:"primaryKey"`
	CreatedAt time.Time  `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt time.Time  `json:"updatedAt" gorm:"autoUpdateTime"`
	DeletedAt *time.Time `json:"-" gorm:"index"`

	// 会话基本信息
	SessionID string        `json:"sessionId" gorm:"type:varchar(64);uniqueIndex;not null"` // 会话ID（业务ID）
	CallID    string        `json:"callId" gorm:"size:128;index;not null"`                  // SIP Call-ID
	Status    SessionStatus `json:"status" gorm:"size:20;default:'starting';index"`         // 会话状态

	// 脚本信息
	ScriptID      uint   `json:"scriptId" gorm:"not null;index"`      // 脚本ID
	ScriptName    string `json:"scriptName" gorm:"size:128;not null"` // 脚本名称（冗余字段，便于查询）
	ScriptVersion string `json:"scriptVersion" gorm:"size:32"`        // 脚本版本

	// 通话信息
	CallerNumber  string `json:"callerNumber,omitempty" gorm:"size:20;index"` // 主叫号码
	CalleeNumber  string `json:"calleeNumber,omitempty" gorm:"size:20;index"` // 被叫号码
	ClientRTPAddr string `json:"clientRtpAddr,omitempty" gorm:"size:128"`     // 客户端RTP地址

	// 执行状态
	CurrentStepID string     `json:"currentStepId,omitempty" gorm:"size:64"` // 当前步骤ID
	StartTime     time.Time  `json:"startTime"`                              // 开始时间
	EndTime       *time.Time `json:"endTime,omitempty"`                      // 结束时间
	Duration      int        `json:"duration" gorm:"default:0"`              // 持续时间（秒）

	// 会话数据
	Context      SessionContext      `json:"context" gorm:"type:json"`      // 会话上下文数据
	Conversation ConversationHistory `json:"conversation" gorm:"type:json"` // 对话历史

	// 执行统计
	TotalSteps     int `json:"totalSteps" gorm:"default:0"`     // 总步骤数
	CompletedSteps int `json:"completedSteps" gorm:"default:0"` // 已完成步骤数
	FailedSteps    int `json:"failedSteps" gorm:"default:0"`    // 失败步骤数

	// 结果信息
	Result       string `json:"result,omitempty" gorm:"type:text"`       // 执行结果
	ErrorMessage string `json:"errorMessage,omitempty" gorm:"type:text"` // 错误信息

	// 音频信息
	RecordingURL  string `json:"recordingUrl,omitempty" gorm:"size:500"` // 录音文件URL
	AudioDuration int    `json:"audioDuration" gorm:"default:0"`         // 音频时长（秒）

	// 质量评估
	QualityScore     float32 `json:"qualityScore" gorm:"default:0"`     // 质量评分（0-100）
	UserSatisfaction int     `json:"userSatisfaction" gorm:"default:0"` // 用户满意度（1-5）

	// 关联关系
	Script         AIPhoneScript   `json:"script,omitempty" gorm:"foreignKey:ScriptID"`
	StepExecutions []StepExecution `json:"stepExecutions,omitempty" gorm:"foreignKey:SessionID;constraint:OnDelete:CASCADE"`
}

// TableName 指定表名
func (AIPhoneSession) TableName() string {
	return constants.TABLE_AI_PHONE_SESSIONS
}

// StepExecution 步骤执行记录表
type StepExecution struct {
	ID        uint       `json:"id" gorm:"primaryKey"`
	CreatedAt time.Time  `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt time.Time  `json:"updatedAt" gorm:"autoUpdateTime"`
	DeletedAt *time.Time `json:"-" gorm:"index"`

	// 关联信息
	SessionID uint     `json:"sessionId" gorm:"not null;index"`      // 会话ID
	StepID    string   `json:"stepId" gorm:"size:64;not null;index"` // 步骤ID
	StepName  string   `json:"stepName" gorm:"size:128;not null"`    // 步骤名称
	StepType  StepType `json:"stepType" gorm:"size:20;not null"`     // 步骤类型

	// 执行信息
	Status     StepExecutionStatus `json:"status" gorm:"size:20;default:'pending';index"` // 执行状态
	StartTime  time.Time           `json:"startTime"`                                     // 开始时间
	EndTime    *time.Time          `json:"endTime,omitempty"`                             // 结束时间
	Duration   int                 `json:"duration" gorm:"default:0"`                     // 执行时长（毫秒）
	RetryCount int                 `json:"retryCount" gorm:"default:0"`                   // 重试次数

	// 输入输出
	Input      string `json:"input,omitempty" gorm:"type:text"`      // 输入数据
	Output     string `json:"output,omitempty" gorm:"type:text"`     // 输出数据
	UserInput  string `json:"userInput,omitempty" gorm:"type:text"`  // 用户输入
	AIResponse string `json:"aiResponse,omitempty" gorm:"type:text"` // AI回复

	// 执行结果
	Result       string `json:"result,omitempty" gorm:"type:text"`       // 执行结果
	ErrorMessage string `json:"errorMessage,omitempty" gorm:"type:text"` // 错误信息
	NextStepID   string `json:"nextStepId,omitempty" gorm:"size:64"`     // 下一步骤ID

	// 音频信息
	AudioFile     string `json:"audioFile,omitempty" gorm:"size:500"` // 音频文件路径
	AudioDuration int    `json:"audioDuration" gorm:"default:0"`      // 音频时长（毫秒）
	TTSText       string `json:"ttsText,omitempty" gorm:"type:text"`  // TTS文本
	ASRText       string `json:"asrText,omitempty" gorm:"type:text"`  // ASR识别文本

	// 质量信息
	Confidence float32 `json:"confidence" gorm:"default:0"` // 置信度（0-1）

	// 关联关系
	Session AIPhoneSession `json:"session,omitempty" gorm:"foreignKey:SessionID"`
}

// TableName 指定表名
func (StepExecution) TableName() string {
	return constants.TABLE_STEP_EXECUTIONS
}

// CRUD 操作函数

// CreateAIPhoneSession 创建AI电话会话
func CreateAIPhoneSession(db *gorm.DB, session *AIPhoneSession) error {
	return db.Create(session).Error
}

// GetAIPhoneSessionByID 根据ID获取会话
func GetAIPhoneSessionByID(db *gorm.DB, id uint) (*AIPhoneSession, error) {
	var session AIPhoneSession
	err := db.Preload("Script").Preload("StepExecutions").First(&session, id).Error
	if err != nil {
		return nil, err
	}
	return &session, nil
}

// GetAIPhoneSessionBySessionID 根据会话ID获取会话
func GetAIPhoneSessionBySessionID(db *gorm.DB, sessionID string) (*AIPhoneSession, error) {
	var session AIPhoneSession
	err := db.Preload("Script").Preload("StepExecutions").Where("session_id = ?", sessionID).First(&session).Error
	if err != nil {
		return nil, err
	}
	return &session, nil
}

// GetAIPhoneSessionByCallID 根据Call ID获取会话
func GetAIPhoneSessionByCallID(db *gorm.DB, callID string) (*AIPhoneSession, error) {
	var session AIPhoneSession
	err := db.Preload("Script").Preload("StepExecutions").Where("call_id = ?", callID).First(&session).Error
	if err != nil {
		return nil, err
	}
	return &session, nil
}

// GetAIPhoneSessionsByScriptID 根据脚本ID获取会话列表
func GetAIPhoneSessionsByScriptID(db *gorm.DB, scriptID uint, limit int) ([]AIPhoneSession, error) {
	var sessions []AIPhoneSession
	query := db.Where("script_id = ?", scriptID).Order("created_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&sessions).Error
	return sessions, err
}

// GetRunningAIPhoneSessions 获取运行中的会话
func GetRunningAIPhoneSessions(db *gorm.DB) ([]AIPhoneSession, error) {
	var sessions []AIPhoneSession
	err := db.Where("status IN ?", []SessionStatus{SessionStatusStarting, SessionStatusRunning}).Find(&sessions).Error
	return sessions, err
}

// UpdateAIPhoneSession 更新会话
func UpdateAIPhoneSession(db *gorm.DB, session *AIPhoneSession) error {
	return db.Save(session).Error
}

// DeleteAIPhoneSession 删除会话（软删除）
func DeleteAIPhoneSession(db *gorm.DB, id uint) error {
	return db.Delete(&AIPhoneSession{}, id).Error
}

// CreateStepExecution 创建步骤执行记录
func CreateStepExecution(db *gorm.DB, execution *StepExecution) error {
	return db.Create(execution).Error
}

// GetStepExecutionsBySessionID 获取会话的所有步骤执行记录
func GetStepExecutionsBySessionID(db *gorm.DB, sessionID uint) ([]StepExecution, error) {
	var executions []StepExecution
	err := db.Where("session_id = ?", sessionID).Order("created_at ASC").Find(&executions).Error
	return executions, err
}

// UpdateStepExecution 更新步骤执行记录
func UpdateStepExecution(db *gorm.DB, execution *StepExecution) error {
	return db.Save(execution).Error
}

// DeleteStepExecution 删除步骤执行记录
func DeleteStepExecution(db *gorm.DB, id uint) error {
	return db.Delete(&StepExecution{}, id).Error
}

// 辅助方法

// IsRunning 检查会话是否运行中
func (s *AIPhoneSession) IsRunning() bool {
	return s.Status == SessionStatusRunning || s.Status == SessionStatusStarting
}

// IsCompleted 检查会话是否已完成
func (s *AIPhoneSession) IsCompleted() bool {
	return s.Status == SessionStatusCompleted
}

// AddMessage 添加对话消息
func (s *AIPhoneSession) AddMessage(role, content, stepID string) {
	message := ConversationMessage{
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
		StepID:    stepID,
	}
	s.Conversation = append(s.Conversation, message)
}

// SetContextValue 设置上下文值
func (s *AIPhoneSession) SetContextValue(key string, value interface{}) {
	if s.Context == nil {
		s.Context = make(SessionContext)
	}
	s.Context[key] = value
}

// GetContextValue 获取上下文值
func (s *AIPhoneSession) GetContextValue(key string) (interface{}, bool) {
	if s.Context == nil {
		return nil, false
	}
	value, exists := s.Context[key]
	return value, exists
}

// CalculateDuration 计算会话持续时间
func (s *AIPhoneSession) CalculateDuration() {
	if s.EndTime != nil {
		s.Duration = int(s.EndTime.Sub(s.StartTime).Seconds())
	}
}

// MarkCompleted 标记会话完成
func (s *AIPhoneSession) MarkCompleted(db *gorm.DB, result string) error {
	now := time.Now()
	s.Status = SessionStatusCompleted
	s.EndTime = &now
	s.Result = result
	s.CalculateDuration()
	return db.Save(s).Error
}

// MarkFailed 标记会话失败
func (s *AIPhoneSession) MarkFailed(db *gorm.DB, errorMessage string) error {
	now := time.Now()
	s.Status = SessionStatusFailed
	s.EndTime = &now
	s.ErrorMessage = errorMessage
	s.CalculateDuration()
	return db.Save(s).Error
}

// IsCompleted 检查步骤是否已完成
func (se *StepExecution) IsCompleted() bool {
	return se.Status == StepStatusCompleted
}

// IsFailed 检查步骤是否失败
func (se *StepExecution) IsFailed() bool {
	return se.Status == StepStatusFailed
}

// MarkCompleted 标记步骤完成
func (se *StepExecution) MarkCompleted(db *gorm.DB, output, nextStepID string) error {
	now := time.Now()
	se.Status = StepStatusCompleted
	se.EndTime = &now
	se.Output = output
	se.NextStepID = nextStepID
	se.Duration = int(now.Sub(se.StartTime).Milliseconds())
	return db.Save(se).Error
}

// MarkFailed 标记步骤失败
func (se *StepExecution) MarkFailed(db *gorm.DB, errorMessage string) error {
	now := time.Now()
	se.Status = StepStatusFailed
	se.EndTime = &now
	se.ErrorMessage = errorMessage
	se.Duration = int(now.Sub(se.StartTime).Milliseconds())
	return db.Save(se).Error
}
