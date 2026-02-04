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
	llmService LLMService  // LLM服务接口
}

// LLMService LLM服务接口
type LLMService interface {
	Query(text string) (string, error)
	Reset()
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

// SetLLMService 设置LLM服务
func (engine *AIPhoneEngine) SetLLMService(llmService LLMService) {
	engine.llmService = llmService
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
		nextStepID, err = engine.executeCollectStep(session, step, execution)
	case models.StepTypeCondition:
		nextStepID, err = engine.executeConditionStep(session, step, execution)
	case models.StepTypeWait:
		nextStepID, err = engine.executeWaitStep(session, step, execution)
	case models.StepTypeDTMF:
		nextStepID, err = engine.executeDTMFStep(session, step, execution)
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

	// 2. 等待并处理用户输入
	maxRetries := 2 // 减少重试次数，给用户更多思考时间
	retryCount := 0
	hasUserInput := false

	// 第一次等待时间更长，给用户充分的反应时间
	initialTimeout := 15 * time.Second
	retryTimeout := 10 * time.Second

	for retryCount <= maxRetries && !hasUserInput {
		currentTimeout := initialTimeout
		if retryCount > 0 {
			currentTimeout = retryTimeout
		}

		logger.Info("Waiting for user response",
			zap.String("call_id", session.CallID),
			zap.Int("attempt", retryCount+1),
			zap.Duration("timeout", currentTimeout))

		// 监听用户输入
		userText, err := engine.listenForUserInput(session, currentTimeout)
		if err != nil {
			logger.Warn("Failed to get user input",
				zap.String("call_id", session.CallID),
				zap.Int("attempt", retryCount+1),
				zap.Error(err))
			retryCount++
			continue
		}

		if userText == "" {
			logger.Info("No user input received",
				zap.String("call_id", session.CallID),
				zap.Int("attempt", retryCount+1))

			retryCount++

			// 如果没有用户输入，播放不同的提示语
			if retryCount <= maxRetries {
				var promptText string
				switch retryCount {
				case 1:
					promptText = "您好，请问您能听到我说话吗？如果能听到请回应一下。"
				case 2:
					promptText = "如果您能听到，请说话或者按任意键。"
				}

				logger.Info("Playing retry prompt",
					zap.String("call_id", session.CallID),
					zap.String("prompt", promptText),
					zap.Int("retry", retryCount))

				if err := engine.playTTSAudio(session, promptText, data.SpeakerID); err != nil {
					logger.Error("Failed to play retry prompt", zap.Error(err))
				}
			}
			continue
		}

		// 有用户输入，标记为成功
		hasUserInput = true

		logger.Info("User input received successfully",
			zap.String("call_id", session.CallID),
			zap.String("input", userText),
			zap.Int("attempt", retryCount+1))

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
			return "", fmt.Errorf("AI service failed: %w", err)
		}

		// 添加AI回复到对话历史
		session.addMessage("assistant", aiResponse, step.StepID)
		execution.AIResponse = aiResponse

		logger.Info("AI response generated",
			zap.String("call_id", session.CallID),
			zap.String("response", aiResponse))

		// 播放AI回复
		if err := engine.playTTSAudio(session, aiResponse, data.SpeakerID); err != nil {
			logger.Error("Failed to play AI response",
				zap.String("call_id", session.CallID),
				zap.Error(err))
			return "", fmt.Errorf("failed to play AI response: %w", err)
		}

		// 更新数据库会话记录
		session.DBSession.Conversation = models.ConversationHistory(session.Conversation)
		session.DBSession.Context = models.SessionContext(session.Context)
		models.UpdateAIPhoneSession(engine.db, session.DBSession)

		// 检查是否需要结束对话
		if engine.shouldEndConversation(aiResponse) {
			logger.Info("Conversation ended by AI response",
				zap.String("call_id", session.CallID),
				zap.String("response", aiResponse))
			break
		}
	}

	// 如果经过多次重试仍然没有用户输入，根据步骤配置决定下一步
	if !hasUserInput {
		logger.Warn("No user input after all attempts",
			zap.String("call_id", session.CallID),
			zap.Int("total_attempts", retryCount))

		// 设置上下文标记，表示用户没有回应
		session.Context["no_user_response"] = true
		session.Context["retry_count"] = retryCount
	} else {
		// 清除之前的无回应标记
		delete(session.Context, "no_user_response")
	}

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

	logger.Debug("Evaluating condition",
		zap.String("call_id", session.CallID),
		zap.String("condition", data.Condition))

	// 评估条件
	result, err := engine.evaluateCondition(session, data.Condition)
	if err != nil {
		return "", fmt.Errorf("failed to evaluate condition: %w", err)
	}

	execution.Input = data.Condition
	execution.Output = fmt.Sprintf("condition result: %t", result)

	logger.Info("Condition evaluation result",
		zap.String("call_id", session.CallID),
		zap.String("condition", data.Condition),
		zap.Bool("result", result))

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

// executeCollectStep 执行收集用户输入步骤
func (engine *AIPhoneEngine) executeCollectStep(session *ScriptSession, step *models.AIPhoneScriptStep, execution *models.StepExecution) (string, error) {
	data := step.Data

	// 播放提示语
	if data.Welcome != "" {
		if err := engine.playTTSAudio(session, data.Welcome, data.SpeakerID); err != nil {
			return "", fmt.Errorf("failed to play prompt: %w", err)
		}
		execution.TTSText = data.Welcome
	}

	// 等待用户输入
	maxRetries := 2
	retryCount := 0

	// 收集步骤给用户更长的思考时间
	initialTimeout := 20 * time.Second
	retryTimeout := 15 * time.Second

	for retryCount <= maxRetries {
		currentTimeout := initialTimeout
		if retryCount > 0 {
			currentTimeout = retryTimeout
		}

		logger.Info("Collecting user input",
			zap.String("call_id", session.CallID),
			zap.Int("attempt", retryCount+1),
			zap.Duration("timeout", currentTimeout))

		userText, err := engine.listenForUserInput(session, currentTimeout)
		if err != nil {
			logger.Warn("Failed to collect user input",
				zap.String("call_id", session.CallID),
				zap.Int("attempt", retryCount+1),
				zap.Error(err))
			retryCount++
			continue
		}

		if userText != "" {
			// 成功收集到用户输入
			session.addMessage("user", userText, step.StepID)
			execution.UserInput = userText
			execution.ASRText = userText

			// 将输入保存到会话上下文中
			if data.CollectKey != "" {
				session.Context[data.CollectKey] = userText
			}

			logger.Info("User input collected successfully",
				zap.String("call_id", session.CallID),
				zap.String("input", userText),
				zap.String("collect_key", data.CollectKey))

			return data.NextStep, nil
		}

		retryCount++

		// 播放重试提示
		if retryCount <= maxRetries {
			var retryPrompt string
			switch retryCount {
			case 1:
				retryPrompt = "抱歉，我没有听清楚您的回答，请您再说一遍。"
			case 2:
				retryPrompt = "请您大声清楚地说出您的回答。"
			}

			logger.Info("Playing collect retry prompt",
				zap.String("call_id", session.CallID),
				zap.String("prompt", retryPrompt),
				zap.Int("retry", retryCount))

			if err := engine.playTTSAudio(session, retryPrompt, data.SpeakerID); err != nil {
				logger.Error("Failed to play retry prompt", zap.Error(err))
			}
		}
	}

	// 收集失败，标记上下文
	session.Context["collect_failed"] = true
	session.Context["collect_retry_count"] = retryCount

	logger.Warn("Failed to collect user input after all attempts",
		zap.String("call_id", session.CallID),
		zap.Int("total_attempts", retryCount))

	// 根据配置决定下一步，如果有失败分支则走失败分支
	if data.FalseNext != "" {
		return data.FalseNext, nil
	}

	return data.NextStep, nil
}

// executeDTMFStep 执行DTMF按键检测步骤
func (engine *AIPhoneEngine) executeDTMFStep(session *ScriptSession, step *models.AIPhoneScriptStep, execution *models.StepExecution) (string, error) {
	data := step.Data

	// 播放DTMF提示语
	if data.DTMFPrompt != "" {
		if err := engine.playTTSAudio(session, data.DTMFPrompt, data.SpeakerID); err != nil {
			return "", fmt.Errorf("failed to play DTMF prompt: %w", err)
		}
		execution.TTSText = data.DTMFPrompt
	}

	// 设置DTMF检测参数
	timeout := time.Duration(data.DTMFTimeout) * time.Millisecond
	if timeout == 0 {
		timeout = 10 * time.Second // 默认10秒超时
	}

	maxDigits := data.DTMFMaxDigits
	if maxDigits == 0 {
		maxDigits = 1 // 默认检测1个按键
	}

	terminator := data.DTMFTerminator
	if terminator == "" {
		terminator = "#" // 默认#作为结束符
	}

	logger.Info("Starting DTMF detection",
		zap.String("call_id", session.CallID),
		zap.Duration("timeout", timeout),
		zap.Int("max_digits", maxDigits),
		zap.String("terminator", terminator))

	// 检测DTMF按键
	dtmfInput, err := engine.listenForDTMF(session, timeout, maxDigits, terminator)
	if err != nil {
		logger.Error("DTMF detection failed",
			zap.String("call_id", session.CallID),
			zap.Error(err))
		return data.FalseNext, nil
	}

	if dtmfInput == "" {
		logger.Info("No DTMF input received",
			zap.String("call_id", session.CallID))
		return data.FalseNext, nil
	}

	// 记录DTMF输入
	execution.UserInput = dtmfInput
	session.addMessage("user", fmt.Sprintf("DTMF: %s", dtmfInput), step.StepID)

	logger.Info("DTMF input received",
		zap.String("call_id", session.CallID),
		zap.String("dtmf", dtmfInput))

	// 根据DTMF选项决定下一步
	if data.DTMFOptions != nil {
		if nextStep, exists := data.DTMFOptions[dtmfInput]; exists {
			return nextStep, nil
		}
	}

	// 如果没有匹配的选项，使用默认下一步
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

// GetScriptByPhoneNumber 根据电话号码获取脚本
func (engine *AIPhoneEngine) GetScriptByPhoneNumber(phoneNumber string) (*models.AIPhoneScript, error) {
	return models.GetAIPhoneScriptByPhone(engine.db, phoneNumber)
}

// CreateSession 创建AI会话
func (engine *AIPhoneEngine) CreateSession(callSid, from, to string, scriptID uint) (*ScriptSession, error) {
	// 获取脚本
	script, err := models.GetAIPhoneScriptByID(engine.db, scriptID)
	if err != nil {
		return nil, fmt.Errorf("failed to get script: %w", err)
	}

	// 创建会话
	sessionID := fmt.Sprintf("twilio_%s", callSid)
	session := &ScriptSession{
		SessionID:    sessionID,
		CallID:       callSid,
		ClientAddr:   from,
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
		return nil, fmt.Errorf("start step not found: %s", script.StartStepID)
	}

	// 创建数据库会话记录
	dbSession := &models.AIPhoneSession{
		SessionID:     sessionID,
		CallID:        callSid,
		Status:        models.SessionStatusStarting,
		ScriptID:      script.ID,
		ScriptName:    script.Name,
		ScriptVersion: script.Version,
		CallerNumber:  from,
		CalleeNumber:  to,
		ClientRTPAddr: from,
		StartTime:     time.Now(),
		Context:       models.SessionContext(session.Context),
		Conversation:  models.ConversationHistory(session.Conversation),
	}

	if err := models.CreateAIPhoneSession(engine.db, dbSession); err != nil {
		return nil, fmt.Errorf("failed to create session record: %w", err)
	}

	session.DBSession = dbSession

	// 保存会话
	engine.mutex.Lock()
	engine.sessions[callSid] = session
	engine.mutex.Unlock()

	logger.Info("Twilio AI session created",
		zap.String("call_sid", callSid),
		zap.String("script", script.Name),
		zap.String("session_id", sessionID))

	return session, nil
}

// GetWelcomeMessage 获取欢迎消息
func (engine *AIPhoneEngine) GetWelcomeMessage(script *models.AIPhoneScript) string {
	// 获取起始步骤
	startStep := script.GetStartStep()
	if startStep == nil {
		return "您好，欢迎致电。"
	}

	// 根据步骤类型返回欢迎消息
	switch startStep.Type {
	case models.StepTypeCallout:
		if startStep.Data.Welcome != "" {
			return startStep.Data.Welcome
		}
		return "您好，我是AI助手，请问有什么可以帮助您的吗？"
	case models.StepTypePlayAudio:
		if startStep.Data.AudioText != "" {
			return startStep.Data.AudioText
		}
		if startStep.Data.Welcome != "" {
			return startStep.Data.Welcome
		}
		return "您好，欢迎致电。"
	default:
		return "您好，欢迎致电。"
	}
}

// ProcessUserInput 处理用户输入
func (engine *AIPhoneEngine) ProcessUserInput(callSid, userInput string) (string, bool, error) {
	// 获取会话
	engine.mutex.RLock()
	session, exists := engine.sessions[callSid]
	engine.mutex.RUnlock()

	if !exists {
		return "抱歉，会话已结束。", false, fmt.Errorf("session not found: %s", callSid)
	}

	// 添加用户消息到对话历史
	session.addMessage("user", userInput, session.CurrentStep.StepID)

	// 根据当前步骤类型处理用户输入
	switch session.CurrentStep.Type {
	case models.StepTypeCallout:
		return engine.processCalloutInput(session, userInput)
	case models.StepTypeCollect:
		return engine.processCollectInput(session, userInput)
	default:
		// 默认使用AI处理
		aiResponse, err := engine.callAIService(session, session.CurrentStep.Data.Prompt)
		if err != nil {
			return "抱歉，系统出现错误。", false, err
		}

		session.addMessage("assistant", aiResponse, session.CurrentStep.StepID)

		// 检查是否应该结束对话
		shouldContinue := !engine.shouldEndConversation(aiResponse)

		return aiResponse, shouldContinue, nil
	}
}

// processCalloutInput 处理对话步骤的用户输入
func (engine *AIPhoneEngine) processCalloutInput(session *ScriptSession, userInput string) (string, bool, error) {
	// 使用AI处理用户输入
	aiResponse, err := engine.callAIService(session, session.CurrentStep.Data.Prompt)
	if err != nil {
		return "抱歉，系统出现错误。", false, err
	}

	// 添加AI回复到对话历史
	session.addMessage("assistant", aiResponse, session.CurrentStep.StepID)

	// 更新数据库会话记录
	session.DBSession.Conversation = models.ConversationHistory(session.Conversation)
	session.DBSession.Context = models.SessionContext(session.Context)
	models.UpdateAIPhoneSession(engine.db, session.DBSession)

	// 检查是否应该结束对话
	shouldContinue := !engine.shouldEndConversation(aiResponse)

	logger.Info("AI response generated for Twilio",
		zap.String("call_sid", session.CallID),
		zap.String("user_input", userInput),
		zap.String("ai_response", aiResponse),
		zap.Bool("should_continue", shouldContinue))

	return aiResponse, shouldContinue, nil
}

// processCollectInput 处理收集步骤的用户输入
func (engine *AIPhoneEngine) processCollectInput(session *ScriptSession, userInput string) (string, bool, error) {
	// 将输入保存到会话上下文中
	if session.CurrentStep.Data.CollectKey != "" {
		session.Context[session.CurrentStep.Data.CollectKey] = userInput
	}

	// 移动到下一步
	nextStepID := session.CurrentStep.Data.NextStep
	if nextStepID != "" {
		session.CurrentStep = session.Script.GetStepByID(nextStepID)
	}

	// 返回确认消息
	response := "好的，我已经记录了您的信息。"
	if session.CurrentStep != nil && session.CurrentStep.Data.Welcome != "" {
		response = session.CurrentStep.Data.Welcome
	}

	session.addMessage("assistant", response, session.CurrentStep.StepID)

	return response, session.CurrentStep != nil, nil
}

// EndSession 结束会话
func (engine *AIPhoneEngine) EndSession(callSid string) error {
	engine.mutex.RLock()
	session, exists := engine.sessions[callSid]
	engine.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("session not found: %s", callSid)
	}

	// 标记会话完成
	session.markCompleted("Call ended by Twilio")

	// 清理会话
	engine.cleanupSession(session)

	logger.Info("Twilio session ended",
		zap.String("call_sid", callSid),
		zap.String("session_id", session.SessionID))

	return nil
}
