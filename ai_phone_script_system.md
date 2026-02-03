# AI电话脚本系统设计

## 核心概念

这是一个**脚本驱动的AI电话系统**，通过JSON配置定义通话流程，AI按照脚本执行任务。

## 脚本结构分析

```json
{
  "name": "青羊出入境办事大厅",           // 脚本名称
  "speakerId": "10001",                  // 说话人ID（TTS音色）
  "startId": "begin",                    // 起始步骤ID
  "groups": [                            // 步骤组
    {
      "id": "begin",                     // 组ID
      "name": "欢迎提示",                // 组名称
      "steps": [                         // 步骤列表
        {
          "id": "SYQEfdVZAe7dTHrs44mABC", // 步骤ID
          "type": "callout",              // 步骤类型
          "data": {
            "prompt": "你是一名成都市金牛区就业局的工作人员...", // AI提示词
            "speakerId": "1",             // TTS音色ID
            "welcome": "你好我是成都市金牛区就业局的工作人员", // 开场白
            "sliceTime": 30000            // 单次对话时长限制(ms)
          }
        }
      ]
    }
  ]
}
```

## 系统架构设计

### 1. 脚本数据结构

```go
// 脚本配置
type AIPhoneScript struct {
    Name      string       `json:"name"`
    SpeakerID string       `json:"speakerId"`
    StartID   string       `json:"startId"`
    Groups    []StepGroup  `json:"groups"`
}

// 步骤组
type StepGroup struct {
    ID    string `json:"id"`
    Name  string `json:"name"`
    Steps []Step `json:"steps"`
}

// 执行步骤
type Step struct {
    ID   string   `json:"id"`
    Type StepType `json:"type"`
    Data StepData `json:"data"`
}

// 步骤类型
type StepType string

const (
    StepTypeCallout    StepType = "callout"    // AI对话
    StepTypePlayAudio  StepType = "playaudio"  // 播放音频
    StepTypeCollect    StepType = "collect"    // 收集信息
    StepTypeTransfer   StepType = "transfer"   // 转接
    StepTypeHangup     StepType = "hangup"     // 挂断
    StepTypeCondition  StepType = "condition"  // 条件判断
)

// 步骤数据
type StepData struct {
    // AI对话相关
    Prompt     string `json:"prompt"`     // AI提示词
    Welcome    string `json:"welcome"`    // 开场白
    SpeakerID  string `json:"speakerId"`  // TTS音色
    SliceTime  int    `json:"sliceTime"`  // 对话时长限制
    
    // 条件判断相关
    Condition  string `json:"condition"`  // 判断条件
    TrueNext   string `json:"trueNext"`   // 条件为真时的下一步
    FalseNext  string `json:"falseNext"`  // 条件为假时的下一步
    
    // 信息收集相关
    CollectKey string `json:"collectKey"` // 收集的信息键名
    Validation string `json:"validation"` // 验证规则
    
    // 音频播放相关
    AudioFile  string `json:"audioFile"`  // 音频文件路径
    
    // 通用
    NextStep   string `json:"nextStep"`   // 下一步骤ID
}
```

### 2. 脚本执行引擎

```go
// 脚本执行会话
type ScriptSession struct {
    CallID       string
    ClientAddr   string
    Script       *AIPhoneScript
    CurrentStep  *Step
    Context      map[string]interface{} // 上下文数据
    Conversation []Message              // 对话历史
    StopChan     chan bool
}

// 脚本执行引擎
type ScriptEngine struct {
    server   *SipServer
    scripts  map[string]*AIPhoneScript // 脚本缓存
    sessions map[string]*ScriptSession // 活跃会话
}

// 启动脚本执行
func (se *ScriptEngine) StartScript(callID, clientAddr, scriptName string) {
    script := se.getScript(scriptName)
    if script == nil {
        logger.Error("Script not found", zap.String("script", scriptName))
        return
    }
    
    session := &ScriptSession{
        CallID:     callID,
        ClientAddr: clientAddr,
        Script:     script,
        Context:    make(map[string]interface{}),
        StopChan:   make(chan bool, 1),
    }
    
    // 找到起始步骤
    session.CurrentStep = se.findStep(script, script.StartID)
    
    // 执行脚本
    go se.executeScript(session)
}

// 执行脚本主循环
func (se *ScriptEngine) executeScript(session *ScriptSession) {
    defer se.cleanupSession(session.CallID)
    
    for session.CurrentStep != nil {
        select {
        case <-session.StopChan:
            return
        default:
            nextStepID := se.executeStep(session, session.CurrentStep)
            if nextStepID == "" {
                break // 脚本结束
            }
            session.CurrentStep = se.findStep(session.Script, nextStepID)
        }
    }
    
    logger.Info("Script execution completed", zap.String("call_id", session.CallID))
}
```

### 3. 步骤执行器

```go
// 执行单个步骤
func (se *ScriptEngine) executeStep(session *ScriptSession, step *Step) string {
    logger.Info("Executing step", 
        zap.String("call_id", session.CallID),
        zap.String("step_id", step.ID),
        zap.String("step_type", string(step.Type)))
    
    switch step.Type {
    case StepTypeCallout:
        return se.executeCalloutStep(session, step)
    case StepTypePlayAudio:
        return se.executePlayAudioStep(session, step)
    case StepTypeCollect:
        return se.executeCollectStep(session, step)
    case StepTypeCondition:
        return se.executeConditionStep(session, step)
    case StepTypeTransfer:
        return se.executeTransferStep(session, step)
    case StepTypeHangup:
        se.executeHangupStep(session, step)
        return "" // 结束脚本
    default:
        logger.Warn("Unknown step type", zap.String("type", string(step.Type)))
        return step.Data.NextStep
    }
}

// 执行AI对话步骤
func (se *ScriptEngine) executeCalloutStep(session *ScriptSession, step *Step) string {
    data := step.Data
    
    // 1. 播放开场白
    if data.Welcome != "" {
        se.playWelcomeMessage(session, data.Welcome, data.SpeakerID)
    }
    
    // 2. 开始AI对话循环
    startTime := time.Now()
    maxDuration := time.Duration(data.SliceTime) * time.Millisecond
    
    for time.Since(startTime) < maxDuration {
        // 监听用户输入
        userText := se.listenForUserInput(session)
        if userText == "" {
            break // 用户无输入或超时
        }
        
        // 添加到对话历史
        session.Conversation = append(session.Conversation, Message{
            Role:    "user",
            Content: userText,
            Time:    time.Now(),
        })
        
        // AI处理
        aiResponse := se.callAIWithPrompt(session.Conversation, data.Prompt)
        session.Conversation = append(session.Conversation, Message{
            Role:    "assistant", 
            Content: aiResponse,
            Time:    time.Now(),
        })
        
        // 播放AI回复
        se.playAIResponse(session, aiResponse, data.SpeakerID)
        
        // 检查是否需要结束对话
        if se.shouldEndConversation(aiResponse) {
            break
        }
    }
    
    return data.NextStep
}

// 执行条件判断步骤
func (se *ScriptEngine) executeConditionStep(session *ScriptSession, step *Step) string {
    data := step.Data
    
    // 根据条件判断逻辑
    result := se.evaluateCondition(session, data.Condition)
    
    if result {
        return data.TrueNext
    } else {
        return data.FalseNext
    }
}

// 执行信息收集步骤
func (se *ScriptEngine) executeCollectStep(session *ScriptSession, step *Step) string {
    data := step.Data
    
    // 收集用户信息
    userInput := se.collectUserInfo(session, data.CollectKey)
    
    // 验证输入
    if se.validateInput(userInput, data.Validation) {
        session.Context[data.CollectKey] = userInput
        return data.NextStep
    } else {
        // 验证失败，重新收集
        return step.ID
    }
}
```

### 4. 脚本管理

```go
// 脚本管理器
type ScriptManager struct {
    scripts map[string]*AIPhoneScript
    mutex   sync.RWMutex
}

// 加载脚本
func (sm *ScriptManager) LoadScript(scriptPath string) error {
    data, err := os.ReadFile(scriptPath)
    if err != nil {
        return err
    }
    
    var script AIPhoneScript
    if err := json.Unmarshal(data, &script); err != nil {
        return err
    }
    
    sm.mutex.Lock()
    defer sm.mutex.Unlock()
    sm.scripts[script.Name] = &script
    
    logger.Info("Script loaded", zap.String("name", script.Name))
    return nil
}

// 根据被叫号码获取脚本
func (sm *ScriptManager) GetScriptByPhone(phoneNumber string) *AIPhoneScript {
    // 这里可以实现号码到脚本的映射逻辑
    // 比如从数据库查询或配置文件映射
    return sm.scripts["default"]
}
```

## 在ACK回调中的集成

```go
func (as *SipServer) handleAck(req *sip.Request, tx sip.ServerTransaction) {
    // ... 现有代码 ...
    
    // 获取被叫号码
    toURI := req.To().Address.User
    
    // 根据号码选择脚本
    scriptName := as.scriptManager.GetScriptNameByPhone(toURI)
    
    // 启动脚本执行
    as.scriptEngine.StartScript(callID, clientRTPAddr, scriptName)
}
```

## 脚本示例

### 就业调查脚本
```json
{
  "name": "就业需求调查",
  "speakerId": "10001",
  "startId": "welcome",
  "groups": [
    {
      "id": "main",
      "name": "主流程",
      "steps": [
        {
          "id": "welcome",
          "type": "callout",
          "data": {
            "prompt": "你是成都市金牛区就业局工作人员，调查市民就业需求。询问对方是否有就业需要。",
            "welcome": "你好，我是成都市金牛区就业局的工作人员",
            "speakerId": "1",
            "sliceTime": 30000,
            "nextStep": "check_need"
          }
        },
        {
          "id": "check_need",
          "type": "condition",
          "data": {
            "condition": "has_job_need",
            "trueNext": "collect_need",
            "falseNext": "ending"
          }
        },
        {
          "id": "collect_need",
          "type": "callout",
          "data": {
            "prompt": "用户有就业需求，询问具体需要：找工作、就业培训还是创业服务",
            "speakerId": "1",
            "sliceTime": 30000,
            "nextStep": "promise_contact"
          }
        },
        {
          "id": "promise_contact",
          "type": "playaudio",
          "data": {
            "welcome": "请保持电话畅通，我们会尽快安排就业服务专员与您联系",
            "speakerId": "1",
            "nextStep": "ending"
          }
        },
        {
          "id": "ending",
          "type": "callout",
          "data": {
            "prompt": "告知用户可前往居住地就近街道或社区便民服务中心，然后道别结束对话",
            "speakerId": "1",
            "sliceTime": 15000,
            "nextStep": "hangup"
          }
        },
        {
          "id": "hangup",
          "type": "hangup",
          "data": {}
        }
      ]
    }
  ]
}
```

## 优势

1. **灵活配置**：通过JSON配置定义通话流程，无需修改代码
2. **可视化管理**：可以开发Web界面来编辑脚本
3. **复用性强**：同一套引擎可以执行不同的脚本
4. **易于维护**：业务逻辑和技术实现分离
5. **扩展性好**：可以轻松添加新的步骤类型

这样的设计是不是更符合你的需求？