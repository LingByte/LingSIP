package sip1

import (
	"fmt"
	"sync"
	"time"

	"github.com/LingByte/LingSIP/internal/models"
	"github.com/LingByte/LingSIP/pkg/logger"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// AIPhoneEngine AI电话脚本执行引擎
type AIPhoneEngine struct {
	server   *SipServer
	db       *gorm.DB
	sessions map[string]*ScriptSession // 活跃会话 callID -> session
	mutex    sync.RWMutex

	// 服务接口
	asrService interface{} // ASR服务接口
	ttsService interface{} // TTS服务接口
	aiService  interface{} // AI服务接口
}

// ScriptSession 脚本执行会话
type ScriptSession struct {
	// 基本信息
	SessionID  string
	CallID     string
	ClientAddr string

	// 脚本信息
	Script      *models.AIPhoneScript
	CurrentStep *models.AIPhoneScriptStep

	// 会话状态
	Status       models.SessionStatus
	Context      map[string]interface{}
	Conversation []models.ConversationMessage

	// 数据库记录
	DBSession *models.AIPhoneSession

	// 控制通道
	StopChan  chan bool
	AudioChan chan []int16 // 音频数据通道

	// 统计信息
	StartTime time.Time
	StepCount int

	// 音频处理
	audioBuffer []int16
	isListening bool

	mutex sync.RWMutex
}

// NewAIPhoneEngine 创建AI电话引擎
func NewAIPhoneEngine(server *SipServer, db *gorm.DB) *AIPhoneEngine {
	return &AIPhoneEngine{
		server:   server,
		db:       db,
		sessions: make(map[string]*ScriptSession),
	}
}

// SetServices 设置服务接口
func (engine *AIPhoneEngine) SetServices(asrService, ttsService, aiService interface{}) {
	engine.asrService = asrService
	engine.ttsService = ttsService
	engine.aiService = aiService
}

// StartScript 启动脚本执行
func (engine *AIPhoneEngine) StartScript(callID, clientAddr, phoneNumber string) error {
	// 根据电话号码获取脚本
	script, err := models.GetAIPhoneScriptByPhone(engine.db, phoneNumber)
	if err != nil {
		logger.Error("Failed to get script by phone",
			zap.String("phone", phoneNumber),
			zap.Error(err))
		return err
	}

	if script == nil {
		logger.Warn("No script found for phone number", zap.String("phone", phoneNumber))
		return fmt.Errorf("no script found for phone number: %s", phoneNumber)
	}

	// 创建会话
	sessionID := fmt.Sprintf("%d", time.Now().UnixNano()) // 使用时间戳作为数字ID
	session := &ScriptSession{
		SessionID:    sessionID,
		CallID:       callID,
		ClientAddr:   clientAddr,
		Script:       script,
		Status:       models.SessionStatusStarting,
		Context:      make(map[string]interface{}),
		Conversation: make([]models.ConversationMessage, 0),
		StopChan:     make(chan bool, 1),
		AudioChan:    make(chan []int16, 100),
		StartTime:    time.Now(),
	}

	// 获取起始步骤
	session.CurrentStep = script.GetStartStep()
	if session.CurrentStep == nil {
		return fmt.Errorf("start step not found: %s", script.StartStepID)
	}

	// 创建数据库会话记录
	dbSession := &models.AIPhoneSession{
		SessionID:     sessionID,
		CallID:        callID,
		Status:        models.SessionStatusStarting,
		ScriptID:      script.ID,
		ScriptName:    script.Name,
		ScriptVersion: script.Version,
		CalleeNumber:  phoneNumber,
		ClientRTPAddr: clientAddr,
		StartTime:     time.Now(),
		Context:       models.SessionContext(session.Context),
		Conversation:  models.ConversationHistory(session.Conversation),
	}

	if err := models.CreateAIPhoneSession(engine.db, dbSession); err != nil {
		logger.Error("Failed to create session record", zap.Error(err))
		return err
	}

	session.DBSession = dbSession

	// 保存会话
	engine.mutex.Lock()
	engine.sessions[callID] = session
	engine.mutex.Unlock()

	// 启动脚本执行
	go engine.executeScript(session)

	logger.Info("AI phone script started",
		zap.String("call_id", callID),
		zap.String("script", script.Name),
		zap.String("session_id", sessionID))

	return nil
}

// executeScript 执行脚本主循环
func (engine *AIPhoneEngine) executeScript(session *ScriptSession) {
	defer engine.cleanupSession(session)

	// 更新会话状态为运行中
	session.Status = models.SessionStatusRunning
	session.DBSession.Status = models.SessionStatusRunning
	models.UpdateAIPhoneSession(engine.db, session.DBSession)

	logger.Info("Script execution started",
		zap.String("call_id", session.CallID),
		zap.String("session_id", session.SessionID))

	// 增加脚本执行次数
	session.Script.IncrementExecuteCount(engine.db)

	// 执行步骤循环
	for session.CurrentStep != nil && session.StepCount < session.Script.MaxSteps {
		select {
		case <-session.StopChan:
			logger.Info("Script execution stopped", zap.String("call_id", session.CallID))
			return
		default:
			// 检查超时
			if time.Since(session.StartTime) > time.Duration(session.Script.MaxDuration)*time.Millisecond {
				logger.Warn("Script execution timeout", zap.String("call_id", session.CallID))
				session.markTimeout("Script execution timeout")
				return
			}

			// 执行当前步骤
			nextStepID, err := engine.executeStep(session, session.CurrentStep)
			if err != nil {
				logger.Error("Step execution failed",
					zap.String("call_id", session.CallID),
					zap.String("step_id", session.CurrentStep.StepID),
					zap.Error(err))
				session.markFailed(fmt.Sprintf("Step execution failed: %v", err))
				return
			}

			session.StepCount++

			// 获取下一步骤
			if nextStepID == "" {
				// 脚本结束
				break
			}

			session.CurrentStep = session.Script.GetStepByID(nextStepID)
			if session.CurrentStep == nil {
				logger.Error("Next step not found",
					zap.String("call_id", session.CallID),
					zap.String("next_step_id", nextStepID))
				session.markFailed(fmt.Sprintf("Next step not found: %s", nextStepID))
				return
			}
		}
	}

	// 脚本执行完成
	session.markCompleted("Script execution completed successfully")
	session.Script.IncrementSuccessCount(engine.db)

	logger.Info("Script execution completed",
		zap.String("call_id", session.CallID),
		zap.String("session_id", session.SessionID),
		zap.Int("steps", session.StepCount))
}

// executeStep 执行单个步骤
func (engine *AIPhoneEngine) executeStep(session *ScriptSession, step *models.AIPhoneScriptStep) (string, error) {
	logger.Info("Executing step",
		zap.String("call_id", session.CallID),
		zap.String("step_id", step.StepID),
		zap.String("step_type", string(step.Type)),
		zap.String("step_name", step.Name))

	// 创建步骤执行记录
	execution := &models.StepExecution{
		SessionID: session.DBSession.ID,
		StepID:    step.StepID,
		StepName:  step.Name,
		StepType:  step.Type,
		Status:    models.StepStatusRunning,
		StartTime: time.Now(),
	}

	if err := models.CreateStepExecution(engine.db, execution); err != nil {
		logger.Error("Failed to create step execution record", zap.Error(err))
	}

	var nextStepID string
	var err error

	// 根据步骤类型执行
	switch step.Type {
	case models.StepTypeCallout:
		nextStepID, err = engine.executeCalloutStep(session, step, execution)
	case models.StepTypePlayAudio:
		nextStepID, err = engine.executePlayAudioStep(session, step, execution)
	case models.StepTypeCollect:
		nextStepID, err = step.Data.NextStep, nil // TODO: 实现收集步骤
	case models.StepTypeCondition:
		nextStepID, err = engine.executeConditionStep(session, step, execution)
	case models.StepTypeWait:
		nextStepID, err = engine.executeWaitStep(session, step, execution)
	case models.StepTypeRecord:
		nextStepID, err = step.Data.NextStep, nil // TODO: 实现录音步骤
	case models.StepTypeTransfer:
		nextStepID, err = step.Data.NextStep, nil // TODO: 实现转接步骤
	case models.StepTypeHangup:
		err = engine.executeHangupStep(session, step, execution)
		nextStepID = "" // 结束脚本
	default:
		err = fmt.Errorf("unknown step type: %s", step.Type)
		nextStepID = step.Data.NextStep
	}

	// 更新步骤执行记录
	if err != nil {
		execution.MarkFailed(engine.db, err.Error())
	} else {
		execution.MarkCompleted(engine.db, "", nextStepID)
	}

	return nextStepID, err
}

// executeCalloutStep 执行AI对话步骤
func (engine *AIPhoneEngine) executeCalloutStep(session *ScriptSession, step *models.AIPhoneScriptStep, execution *models.StepExecution) (string, error) {
	data := step.Data

	// 1. 播放开场白
	if data.Welcome != "" {
		if err := engine.playTTSAudio(session, data.Welcome, data.SpeakerID); err != nil {
			return "", fmt.Errorf("failed to play welcome message: %w", err)
		}
		execution.TTSText = data.Welcome
	}

	// 2. 开始AI对话循环
	startTime := time.Now()
	maxDuration := time.Duration(data.SliceTime) * time.Millisecond
	if maxDuration == 0 {
		maxDuration = 30 * time.Second // 默认30秒
	}

	conversationCount := 0
	maxConversations := 5 // 最大对话轮数

	for time.Since(startTime) < maxDuration && conversationCount < maxConversations {
		// 监听用户输入
		userText, err := engine.listenForUserInput(session, 10*time.Second)
		if err != nil {
			logger.Warn("Failed to get user input",
				zap.String("call_id", session.CallID),
				zap.Error(err))
			break
		}

		if userText == "" {
			logger.Info("No user input received", zap.String("call_id", session.CallID))
			break
		}

		// 添加用户消息到对话历史
		session.addMessage("user", userText, step.StepID)
		execution.UserInput = userText
		execution.ASRText = userText

		// AI处理
		aiResponse, err := engine.callAIService(session, data.Prompt)
		if err != nil {
			logger.Error("AI service call failed",
				zap.String("call_id", session.CallID),
				zap.Error(err))
			break
		}

		// 添加AI回复到对话历史
		session.addMessage("assistant", aiResponse, step.StepID)
		execution.AIResponse = aiResponse

		// 播放AI回复
		if err := engine.playTTSAudio(session, aiResponse, data.SpeakerID); err != nil {
			logger.Error("Failed to play AI response",
				zap.String("call_id", session.CallID),
				zap.Error(err))
			break
		}

		conversationCount++

		// 检查是否需要结束对话
		if engine.shouldEndConversation(aiResponse) {
			break
		}
	}

	// 更新数据库会话记录
	session.DBSession.Conversation = models.ConversationHistory(session.Conversation)
	session.DBSession.Context = models.SessionContext(session.Context)
	models.UpdateAIPhoneSession(engine.db, session.DBSession)

	return data.NextStep, nil
}

// executePlayAudioStep 执行播放音频步骤
func (engine *AIPhoneEngine) executePlayAudioStep(session *ScriptSession, step *models.AIPhoneScriptStep, execution *models.StepExecution) (string, error) {
	data := step.Data

	var audioText string
	if data.AudioText != "" {
		audioText = data.AudioText
	} else if data.Welcome != "" {
		audioText = data.Welcome
	} else {
		return "", fmt.Errorf("no audio text provided")
	}

	// 播放音频
	if err := engine.playTTSAudio(session, audioText, data.SpeakerID); err != nil {
		return "", fmt.Errorf("failed to play audio: %w", err)
	}

	execution.TTSText = audioText
	return data.NextStep, nil
}

// executeConditionStep 执行条件判断步骤
func (engine *AIPhoneEngine) executeConditionStep(session *ScriptSession, step *models.AIPhoneScriptStep, execution *models.StepExecution) (string, error) {
	data := step.Data

	// 评估条件
	result, err := engine.evaluateCondition(session, data.Condition)
	if err != nil {
		return "", fmt.Errorf("failed to evaluate condition: %w", err)
	}

	execution.Input = data.Condition
	execution.Output = fmt.Sprintf("condition result: %t", result)

	if result {
		return data.TrueNext, nil
	} else {
		return data.FalseNext, nil
	}
}

// executeWaitStep 执行等待步骤
func (engine *AIPhoneEngine) executeWaitStep(session *ScriptSession, step *models.AIPhoneScriptStep, execution *models.StepExecution) (string, error) {
	data := step.Data

	waitTime := time.Duration(data.WaitTime) * time.Millisecond
	if waitTime == 0 {
		waitTime = 1 * time.Second // 默认1秒
	}

	logger.Info("Waiting",
		zap.String("call_id", session.CallID),
		zap.Duration("duration", waitTime))

	select {
	case <-time.After(waitTime):
		// 等待完成
	case <-session.StopChan:
		return "", fmt.Errorf("session stopped during wait")
	}

	return data.NextStep, nil
}

// executeHangupStep 执行挂断步骤
func (engine *AIPhoneEngine) executeHangupStep(session *ScriptSession, step *models.AIPhoneScriptStep, execution *models.StepExecution) error {
	logger.Info("Hanging up call", zap.String("call_id", session.CallID))

	// 这里可以发送BYE请求来主动挂断电话
	// 目前只是标记会话结束
	session.StopChan <- true

	return nil
}

// 辅助方法

// cleanupSession 清理会话
func (engine *AIPhoneEngine) cleanupSession(session *ScriptSession) {
	engine.mutex.Lock()
	delete(engine.sessions, session.CallID)
	engine.mutex.Unlock()

	// 关闭通道
	close(session.StopChan)
	close(session.AudioChan)

	logger.Info("Session cleaned up",
		zap.String("call_id", session.CallID),
		zap.String("session_id", session.SessionID))
}

// addMessage 添加对话消息
func (session *ScriptSession) addMessage(role, content, stepID string) {
	session.mutex.Lock()
	defer session.mutex.Unlock()

	message := models.ConversationMessage{
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
		StepID:    stepID,
	}
	session.Conversation = append(session.Conversation, message)
}

// markCompleted 标记会话完成
func (session *ScriptSession) markCompleted(result string) {
	session.Status = models.SessionStatusCompleted
	session.DBSession.MarkCompleted(nil, result) // 这里应该传入db实例
}

// markFailed 标记会话失败
func (session *ScriptSession) markFailed(errorMessage string) {
	session.Status = models.SessionStatusFailed
	session.DBSession.MarkFailed(nil, errorMessage) // 这里应该传入db实例
}

// markTimeout 标记会话超时
func (session *ScriptSession) markTimeout(errorMessage string) {
	session.Status = models.SessionStatusTimeout
	session.DBSession.Status = models.SessionStatusTimeout
	session.DBSession.ErrorMessage = errorMessage
	// 更新数据库记录
}

// StopSession 停止会话
func (engine *AIPhoneEngine) StopSession(callID string) {
	engine.mutex.RLock()
	session, exists := engine.sessions[callID]
	engine.mutex.RUnlock()

	if exists {
		session.StopChan <- true
		logger.Info("Session stop requested", zap.String("call_id", callID))
	}
}

// GetSession 获取会话
func (engine *AIPhoneEngine) GetSession(callID string) *ScriptSession {
	engine.mutex.RLock()
	defer engine.mutex.RUnlock()
	return engine.sessions[callID]
}
