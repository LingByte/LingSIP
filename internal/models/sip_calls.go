package models

import (
	"time"

	"github.com/LingByte/LingSIP/pkg/constants"
	"gorm.io/gorm"
)

type SipCallStatus string

const (
	SipCallStatusCalling   SipCallStatus = "calling"   // 呼叫中
	SipCallStatusRinging   SipCallStatus = "ringing"   // 响铃中
	SipCallStatusAnswered  SipCallStatus = "answered"  // 已接通
	SipCallStatusFailed    SipCallStatus = "failed"    // 失败
	SipCallStatusCancelled SipCallStatus = "cancelled" // 已取消
	SipCallStatusEnded     SipCallStatus = "ended"     // 已结束
)

// SipCallDirection 通话方向
type SipCallDirection string

const (
	SipCallDirectionInbound  SipCallDirection = "inbound"  // 呼入
	SipCallDirectionOutbound SipCallDirection = "outbound" // 呼出
)

// SipCall SIP通话记录表
type SipCall struct {
	ID                  uint             `json:"id" gorm:"primaryKey"`
	CreatedAt           time.Time        `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt           time.Time        `json:"updatedAt" gorm:"autoUpdateTime"`
	DeletedAt           *time.Time       `json:"-" gorm:"index"`
	CallID              string           `json:"callId" gorm:"size:128;index;not null"`        // SIP Call-ID
	Direction           SipCallDirection `json:"direction" gorm:"size:20;index"`               // 通话方向
	Status              SipCallStatus    `json:"status" gorm:"size:20;index"`                  // 通话状态
	FromUsername        string           `json:"fromUsername,omitempty" gorm:"size:128"`       // 主叫用户名
	FromURI             string           `json:"fromUri,omitempty" gorm:"size:256"`            // 主叫URI
	FromIP              string           `json:"fromIp,omitempty" gorm:"size:64"`              // 主叫IP
	ToUsername          string           `json:"toUsername,omitempty" gorm:"size:128"`         // 被叫用户名
	ToURI               string           `json:"toUri,omitempty" gorm:"size:256"`              // 被叫URI
	ToIP                string           `json:"toIp,omitempty" gorm:"size:64"`                // 被叫IP
	LocalRTPAddr        string           `json:"localRtpAddr,omitempty" gorm:"size:128"`       // 本地RTP地址
	RemoteRTPAddr       string           `json:"remoteRtpAddr,omitempty" gorm:"size:128"`      // 远程RTP地址
	StartTime           time.Time        `json:"startTime"`                                    // 开始时间
	AnswerTime          *time.Time       `json:"answerTime,omitempty"`                         // 接通时间
	EndTime             *time.Time       `json:"endTime,omitempty"`                            // 结束时间
	Duration            int              `json:"duration" gorm:"default:0"`                    // 通话时长（秒）
	ErrorCode           int              `json:"errorCode,omitempty"`                          // 错误代码
	ErrorMessage        string           `json:"errorMessage,omitempty" gorm:"size:500"`       // 错误消息
	RecordURL           string           `json:"recordUrl,omitempty" gorm:"size:500"`          // 通话录音文件URL
	Transcription       string           `json:"transcription,omitempty" gorm:"type:text"`     // 转录文本
	TranscriptionStatus string           `json:"transcriptionStatus,omitempty" gorm:"size:20"` // 转录状态：pending, processing, completed, failed
	TranscriptionError  string           `json:"transcriptionError,omitempty" gorm:"size:500"` // 转录错误信息
	Metadata            string           `json:"metadata,omitempty" gorm:"type:text"`          // JSON格式的额外信息
	Notes               string           `json:"notes,omitempty" gorm:"type:text"`             // 备注
}

// TableName get tables
func (SipCall) TableName() string {
	return constants.TABLE_SIP_CALLS
}

// CreateSipCall 创建SIP通话记录
func CreateSipCall(db *gorm.DB, sipCall *SipCall) error {
	return db.Create(sipCall).Error
}

// GetSipCallByCallID 根据CallID获取通话记录
func GetSipCallByCallID(db *gorm.DB, callID string) (*SipCall, error) {
	var sipCall SipCall
	err := db.Where("call_id = ?", callID).First(&sipCall).Error
	if err != nil {
		return nil, err
	}
	return &sipCall, nil
}

// UpdateSipCall 更新SIP通话记录
func UpdateSipCall(db *gorm.DB, sipCall *SipCall) error {
	return db.Save(sipCall).Error
}

// GetSipCallsByUserID 根据用户ID获取通话记录列表
func GetSipCallsByUserID(db *gorm.DB, userID uint, limit int) ([]SipCall, error) {
	var sipCalls []SipCall
	query := db.Where("user_id = ?", userID).Order("created_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&sipCalls).Error
	return sipCalls, err
}

// GetSipCallsByStatus 根据状态获取通话记录列表
func GetSipCallsByStatus(db *gorm.DB, status SipCallStatus, limit int) ([]SipCall, error) {
	var sipCalls []SipCall
	query := db.Where("status = ?", status).Order("created_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&sipCalls).Error
	return sipCalls, err
}
