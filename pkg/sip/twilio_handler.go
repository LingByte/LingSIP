package sip1

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/LingByte/LingSIP/pkg/logger"
	"go.uber.org/zap"
)

// TwiMLResponse TwiML响应结构
type TwiMLResponse struct {
	XMLName xml.Name `xml:"Response"`
	Say     []Say    `xml:"Say,omitempty"`
	Gather  *Gather  `xml:"Gather,omitempty"`
	Hangup  *Hangup  `xml:"Hangup,omitempty"`
	Pause   *Pause   `xml:"Pause,omitempty"`
}

// Say TwiML Say动词
type Say struct {
	Voice    string `xml:"voice,attr,omitempty"`
	Language string `xml:"language,attr,omitempty"`
	Text     string `xml:",chardata"`
}

// Gather TwiML Gather动词
type Gather struct {
	Input     string `xml:"input,attr,omitempty"`
	Action    string `xml:"action,attr,omitempty"`
	Method    string `xml:"method,attr,omitempty"`
	Timeout   int    `xml:"timeout,attr,omitempty"`
	NumDigits int    `xml:"numDigits,attr,omitempty"`
	Say       *Say   `xml:"Say,omitempty"`
}

// Hangup TwiML Hangup动词
type Hangup struct{}

// Pause TwiML Pause动词
type Pause struct {
	Length int `xml:"length,attr,omitempty"`
}

// TwilioWebhookHandler Twilio Webhook处理器
type TwilioWebhookHandler struct {
	aiEngine *AIPhoneEngine
	baseURL  string
}

// NewTwilioWebhookHandler 创建Twilio Webhook处理器
func NewTwilioWebhookHandler(aiEngine *AIPhoneEngine, baseURL string) *TwilioWebhookHandler {
	return &TwilioWebhookHandler{
		aiEngine: aiEngine,
		baseURL:  baseURL,
	}
}

// HandleVoiceWebhook 处理语音Webhook
func (h *TwilioWebhookHandler) HandleVoiceWebhook(w http.ResponseWriter, r *http.Request) {
	// 解析表单数据
	if err := r.ParseForm(); err != nil {
		logger.Error("Failed to parse form data", zap.Error(err))
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// 获取通话信息
	callSid := r.FormValue("CallSid")
	from := r.FormValue("From")
	to := r.FormValue("To")
	callStatus := r.FormValue("CallStatus")

	logger.Info("Twilio voice webhook received",
		zap.String("callSid", callSid),
		zap.String("from", from),
		zap.String("to", to),
		zap.String("callStatus", callStatus))

	// 根据通话状态处理
	switch callStatus {
	case "ringing":
		h.handleIncomingCall(w, r, callSid, from, to)
	case "in-progress":
		h.handleCallInProgress(w, r, callSid)
	case "completed":
		h.handleCallCompleted(callSid)
	default:
		h.handleIncomingCall(w, r, callSid, from, to)
	}
}

// handleIncomingCall 处理来电
func (h *TwilioWebhookHandler) handleIncomingCall(w http.ResponseWriter, r *http.Request, callSid, from, to string) {
	// 查找对应的AI脚本
	script, err := h.aiEngine.GetScriptByPhoneNumber(to)
	if err != nil {
		logger.Error("No script found for phone number",
			zap.String("phoneNumber", to),
			zap.Error(err))

		// 返回默认响应
		response := &TwiMLResponse{
			Say: []Say{{
				Voice:    "alice",
				Language: "zh-CN",
				Text:     "抱歉，当前服务不可用，请稍后再试。",
			}},
			Hangup: &Hangup{},
		}
		h.sendTwiMLResponse(w, response)
		return
	}

	// 创建AI会话
	session, err := h.aiEngine.CreateSession(callSid, from, to, script.ID)
	if err != nil {
		logger.Error("Failed to create AI session", zap.Error(err))
		h.sendErrorResponse(w)
		return
	}

	// 获取欢迎消息
	welcomeMessage := h.aiEngine.GetWelcomeMessage(script)

	// 构建TwiML响应
	response := &TwiMLResponse{
		Gather: &Gather{
			Input:   "speech",
			Action:  fmt.Sprintf("%s/webhook/twilio/gather", h.baseURL),
			Method:  "POST",
			Timeout: 10,
			Say: &Say{
				Voice:    "alice",
				Language: "zh-CN",
				Text:     welcomeMessage,
			},
		},
	}

	logger.Info("AI session created",
		zap.String("sessionId", session.SessionID),
		zap.String("scriptName", script.Name))

	h.sendTwiMLResponse(w, response)
}

// handleCallInProgress 处理通话进行中
func (h *TwilioWebhookHandler) handleCallInProgress(w http.ResponseWriter, r *http.Request, callSid string) {
	// 获取用户输入
	speechResult := r.FormValue("SpeechResult")
	digits := r.FormValue("Digits")

	userInput := speechResult
	if userInput == "" {
		userInput = digits
	}

	logger.Info("User input received",
		zap.String("callSid", callSid),
		zap.String("input", userInput))

	// 处理AI对话
	aiResponse, shouldContinue, err := h.aiEngine.ProcessUserInput(callSid, userInput)
	if err != nil {
		logger.Error("Failed to process user input", zap.Error(err))
		h.sendErrorResponse(w)
		return
	}

	// 构建响应
	var response *TwiMLResponse
	if shouldContinue {
		response = &TwiMLResponse{
			Gather: &Gather{
				Input:   "speech",
				Action:  fmt.Sprintf("%s/webhook/twilio/gather", h.baseURL),
				Method:  "POST",
				Timeout: 10,
				Say: &Say{
					Voice:    "alice",
					Language: "zh-CN",
					Text:     aiResponse,
				},
			},
		}
	} else {
		response = &TwiMLResponse{
			Say: []Say{{
				Voice:    "alice",
				Language: "zh-CN",
				Text:     aiResponse,
			}},
			Pause:  &Pause{Length: 1},
			Hangup: &Hangup{},
		}
	}

	h.sendTwiMLResponse(w, response)
}

// handleCallCompleted 处理通话结束
func (h *TwilioWebhookHandler) handleCallCompleted(callSid string) {
	logger.Info("Call completed", zap.String("callSid", callSid))

	// 结束AI会话
	if err := h.aiEngine.EndSession(callSid); err != nil {
		logger.Error("Failed to end AI session",
			zap.String("callSid", callSid),
			zap.Error(err))
	}
}

// HandleGatherWebhook 处理Gather Webhook
func (h *TwilioWebhookHandler) HandleGatherWebhook(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		logger.Error("Failed to parse form data", zap.Error(err))
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	callSid := r.FormValue("CallSid")
	h.handleCallInProgress(w, r, callSid)
}

// sendTwiMLResponse 发送TwiML响应
func (h *TwilioWebhookHandler) sendTwiMLResponse(w http.ResponseWriter, response *TwiMLResponse) {
	w.Header().Set("Content-Type", "application/xml")

	xmlData, err := xml.MarshalIndent(response, "", "  ")
	if err != nil {
		logger.Error("Failed to marshal TwiML response", zap.Error(err))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// 添加XML声明
	xmlResponse := `<?xml version="1.0" encoding="UTF-8"?>` + "\n" + string(xmlData)

	logger.Debug("Sending TwiML response", zap.String("twiml", xmlResponse))

	w.Write([]byte(xmlResponse))
}

// sendErrorResponse 发送错误响应
func (h *TwilioWebhookHandler) sendErrorResponse(w http.ResponseWriter) {
	response := &TwiMLResponse{
		Say: []Say{{
			Voice:    "alice",
			Language: "zh-CN",
			Text:     "抱歉，系统出现错误，请稍后再试。",
		}},
		Hangup: &Hangup{},
	}
	h.sendTwiMLResponse(w, response)
}

// ValidateTwilioRequest 验证Twilio请求签名（可选）
func (h *TwilioWebhookHandler) ValidateTwilioRequest(r *http.Request, authToken string) bool {
	// 这里可以实现Twilio请求签名验证
	// 参考: https://www.twilio.com/docs/usage/webhooks/webhooks-security
	return true // 暂时跳过验证
}

// TwilioCallInfo Twilio通话信息
type TwilioCallInfo struct {
	CallSid       string
	From          string
	To            string
	CallStatus    string
	Direction     string
	ForwardedFrom string
	CallerName    string
}

// ParseTwilioWebhook 解析Twilio Webhook数据
func ParseTwilioWebhook(values url.Values) *TwilioCallInfo {
	return &TwilioCallInfo{
		CallSid:       values.Get("CallSid"),
		From:          values.Get("From"),
		To:            values.Get("To"),
		CallStatus:    values.Get("CallStatus"),
		Direction:     values.Get("Direction"),
		ForwardedFrom: values.Get("ForwardedFrom"),
		CallerName:    values.Get("CallerName"),
	}
}

// FormatPhoneNumber 格式化电话号码
func FormatPhoneNumber(phoneNumber string) string {
	// 移除+号和空格
	cleaned := strings.ReplaceAll(phoneNumber, "+", "")
	cleaned = strings.ReplaceAll(cleaned, " ", "")
	cleaned = strings.ReplaceAll(cleaned, "-", "")
	return cleaned
}
