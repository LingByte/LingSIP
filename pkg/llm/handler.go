package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sync"

	"github.com/google/uuid"
	"github.com/sashabaranov/go-openai"
	"github.com/sirupsen/logrus"
)

// TTSWriter interface for writing TTS content
type TTSWriter interface {
	Write(delta string, endOfStream, autoHangup bool) error
	GetPlayID() string
}

// Tools interface for handling tool calls
type Tools interface {
	HandleHangup(reason string) error
	HandleRefer() error
}

// SegmentTTSWriter implements TTSWriter for segment-based TTS
type SegmentTTSWriter struct {
	client           TTSClient
	playID           string
	buffer           string
	punctuationRegex *regexp.Regexp
	logger           *logrus.Logger
}

// TTSClient interface for TTS operations
type TTSClient interface {
	TTS(text, voice, playID string, endOfStream, autoHangup bool, onStart, onEnd func(), interrupt bool) error
	StreamTTS(text, voice, playID string, endOfStream, autoHangup bool, onStart, onEnd func(), interrupt bool) error
	Hangup(reason string) error
	Refer(caller, target string, headers map[string]string) error
}

// NewSegmentTTSWriter creates a new segment-based TTS writer
func NewSegmentTTSWriter(client TTSClient, playID string, logger *logrus.Logger) *SegmentTTSWriter {
	return &SegmentTTSWriter{
		client:           client,
		playID:           playID,
		punctuationRegex: regexp.MustCompile(`([.,;:!?，。！？；：])\s*`),
		logger:           logger,
	}
}

func (w *SegmentTTSWriter) Write(delta string, endOfStream, autoHangup bool) error {
	w.buffer += delta

	if endOfStream {
		err := w.client.TTS(w.buffer, "", w.playID, true, autoHangup, nil, nil, false)
		w.buffer = "" // Clear buffer after sending
		return err
	}

	// Check for punctuation in the buffer
	matches := w.punctuationRegex.FindAllStringSubmatchIndex(w.buffer, -1)
	if len(matches) > 0 {
		lastIdx := 0
		for _, match := range matches {
			// Extract the segment up to and including the punctuation
			segment := w.buffer[lastIdx:match[1]]
			if segment != "" {
				// Send this segment to TTS with endOfStream=false (not the final segment)
				if err := w.client.TTS(segment, "", w.playID, false, false, nil, nil, false); err != nil {
					w.logger.WithError(err).Error("Failed to send TTS segment")
					return err
				}
			}
			lastIdx = match[1]
		}

		// Keep the remainder in the buffer
		if lastIdx < len(w.buffer) {
			w.buffer = w.buffer[lastIdx:]
		} else {
			w.buffer = ""
		}
	}

	return nil
}

func (w *SegmentTTSWriter) GetPlayID() string {
	return w.playID
}

// StreamingTTSWriter implements TTSWriter for streaming TTS
type StreamingTTSWriter struct {
	client TTSClient
	playID string
	logger *logrus.Logger
}

// NewStreamingTTSWriter creates a new streaming TTS writer
func NewStreamingTTSWriter(client TTSClient, playID string, logger *logrus.Logger) *StreamingTTSWriter {
	return &StreamingTTSWriter{
		client: client,
		playID: playID,
		logger: logger,
	}
}

func (w *StreamingTTSWriter) Write(delta string, endOfStream, autoHangup bool) error {
	// Don't send empty content unless it's specifically needed for endOfStream signaling
	if delta == "" {
		return nil
	}
	return w.client.StreamTTS(delta, "", w.playID, endOfStream, autoHangup, nil, nil, false)
}

func (w *StreamingTTSWriter) GetPlayID() string {
	return w.playID
}

// DefaultTools implements Tools interface
type DefaultTools struct {
	client      TTSClient
	logger      *logrus.Logger
	referTarget string
	referCaller string
}

func NewDefaultTools(client TTSClient, logger *logrus.Logger, referTarget, referCaller string) *DefaultTools {
	return &DefaultTools{
		client:      client,
		logger:      logger,
		referTarget: referTarget,
		referCaller: referCaller,
	}
}

func (t *DefaultTools) HandleHangup(reason string) error {
	t.logger.WithField("reason", reason).Info("LLM requested hangup")
	return t.client.Hangup(reason)
}

func (t *DefaultTools) HandleRefer() error {
	t.logger.WithField("referTarget", t.referTarget).Info("LLM requested refer")
	return t.client.Refer(t.referCaller, t.referTarget, nil)
}

// LLMHandler manages interactions with OpenAI
type LLMHandler struct {
	client      *openai.Client
	systemMsg   string
	mutex       sync.Mutex
	logger      *logrus.Logger
	ctx         context.Context
	messages    []openai.ChatCompletionMessage
	hangupChan  chan struct{}
	interruptCh chan struct{}
	ReferTarget string
}

// ToolCall represents a function call from the LLM
type HangupTool struct {
	Reason string `json:"reason"`
}

// Define the function for hanging up
var hangupDefinition = openai.FunctionDefinition{
	Name:        "hangup",
	Description: "End the conversation and hang up the call",
	Parameters: json.RawMessage(`{
		"type": "object",
		"properties": {
			"reason": {
				"type": "string",
				"description": "Reason for hanging up the call"
			}
		},
		"required": []
	}`),
}

var referDefinition = openai.FunctionDefinition{
	Name:        "refer",
	Description: "Refer the call to another target",
	Parameters: json.RawMessage(`{
		"type": "object",
		"properties": {},
		"required": []
	}`),
}

// NewLLMHandler creates a new LLM handler
func NewLLMHandler(ctx context.Context, apiKey, endpoint, systemPrompt string, logger *logrus.Logger) *LLMHandler {
	config := openai.DefaultConfig(apiKey)
	config.BaseURL = endpoint
	client := openai.NewClientWithConfig(config)

	// Create system message
	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: systemPrompt,
		},
	}

	return &LLMHandler{
		client:      client,
		systemMsg:   systemPrompt,
		logger:      logger,
		ctx:         ctx,
		messages:    messages,
		hangupChan:  make(chan struct{}),
		interruptCh: make(chan struct{}, 1),
	}
}

// QueryStream processes the LLM response as a stream and sends segments to TTS as they arrive
func (h *LLMHandler) QueryStream(model, text string, streamingTTS bool, client TTSClient, referCaller string) (string, error) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	// Add user message to history
	h.messages = append(h.messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: text,
	})

	// Construct the OpenAI request
	if model == "" {
		model = "qwen-plus"
	}

	request := openai.ChatCompletionRequest{
		Model:       model,
		Messages:    h.messages,
		Temperature: 0.7,
		Stream:      true,
		Tools: []openai.Tool{
			{
				Type:     openai.ToolTypeFunction,
				Function: &hangupDefinition,
			},
		},
	}

	if h.ReferTarget != "" {
		request.Tools = append(request.Tools, openai.Tool{
			Type:     openai.ToolTypeFunction,
			Function: &referDefinition,
		})
	}

	// Generate a unique playID for this conversation
	playID := fmt.Sprintf("llm-%s", uuid.New().String())
	h.logger.WithField("playID", playID).Info("Starting LLM stream with playID")

	// Create appropriate TTS writer based on streaming mode
	var ttsWriter TTSWriter
	if streamingTTS {
		ttsWriter = NewStreamingTTSWriter(client, playID, h.logger)
	} else {
		ttsWriter = NewSegmentTTSWriter(client, playID, h.logger)
	}

	// Create tools handler
	tools := NewDefaultTools(client, h.logger, h.ReferTarget, referCaller)

	// Stream for handling responses
	stream, err := h.client.CreateChatCompletionStream(h.ctx, request)
	if err != nil {
		return "", fmt.Errorf("error creating chat completion stream: %w", err)
	}
	defer stream.Close()

	fullResponse := ""
	var shouldHangup bool
	var shouldRefer bool
	hasTextBeforeHangup := false

	// Process the stream of responses
	for {
		response, err := stream.Recv()
		if err != nil {
			if err.Error() == "EOF" {
				// Stream closed normally - send any remaining content as final segment
				break
			}
			return "", fmt.Errorf("error receiving from stream: %w", err)
		}

		// Check for finish reason to determine if this is the end
		var isFinished bool
		if len(response.Choices) > 0 && response.Choices[0].FinishReason != "" {
			h.logger.WithField("finishReason", response.Choices[0].FinishReason).Debug("LLM stream finished")
			isFinished = true
		}

		// Check for function calls (hangup/refer)
		if len(response.Choices) > 0 && len(response.Choices[0].Delta.ToolCalls) > 0 {
			for _, toolCall := range response.Choices[0].Delta.ToolCalls {
				if toolCall.Function.Name == "hangup" {
					shouldHangup = true
					// If there was text before hangup, send it with endOfStream=true and autoHangup=true
					if hasTextBeforeHangup {
						if err := ttsWriter.Write("", true, true); err != nil {
							h.logger.WithError(err).Error("Failed to send final TTS before hangup")
						} else {
							continue
						}
					}
					// Call tools handler directly for hangup
					if err := tools.HandleHangup("LLM requested hangup"); err != nil {
						h.logger.WithError(err).Error("Failed to handle hangup")
					}
				}
				if toolCall.Function.Name == "refer" {
					shouldRefer = true
					// If there was text before refer, send it with endOfStream=true
					if hasTextBeforeHangup {
						if err := ttsWriter.Write("", true, false); err != nil {
							h.logger.WithError(err).Error("Failed to send final TTS before refer")
						}
					}
					// Call tools handler directly for refer
					if err := tools.HandleRefer(); err != nil {
						h.logger.WithError(err).Error("Failed to handle refer")
					}
				}
			}
		}

		// Process content if available and not in hangup/refer mode
		if len(response.Choices) > 0 && response.Choices[0].Delta.Content != "" && !shouldHangup && !shouldRefer {
			content := response.Choices[0].Delta.Content
			fullResponse += content
			hasTextBeforeHangup = true

			// If this is the final chunk (finished), send with endOfStream=true
			if isFinished {
				if err := ttsWriter.Write(content, true, false); err != nil {
					h.logger.WithError(err).Error("Failed to write final TTS delta")
				}
			} else {
				// Write delta to TTS writer
				if err := ttsWriter.Write(content, false, false); err != nil {
					h.logger.WithError(err).Error("Failed to write TTS delta")
				}
			}
		}

		// If stream finished, break after processing the final content
		if isFinished {
			break
		}
	}

	// Send final buffered content if not already handled by tool calls or finish reason
	if !shouldHangup && !shouldRefer && hasTextBeforeHangup {
		// Flush any remaining buffered content as final segment
		if err := ttsWriter.Write("", true, false); err != nil {
			h.logger.WithError(err).Error("Failed to flush final TTS buffer")
		}
	}

	// Update conversation history
	if shouldHangup {
		h.messages = append(h.messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleAssistant,
			Content: "user requested hangup",
		})
	} else if shouldRefer {
		h.messages = append(h.messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleAssistant,
			Content: "user requested refer",
		})
	} else {
		h.messages = append(h.messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleAssistant,
			Content: fullResponse,
		})
	}

	h.logger.WithFields(logrus.Fields{
		"fullResponse": fullResponse,
		"hangup":       shouldHangup,
		"refer":        shouldRefer,
	}).Info("LLM stream completed")

	return fullResponse, nil
}

// Query processes a simple LLM query without streaming
func (h *LLMHandler) Query(model, text string) (string, error) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	// Add user message to history
	h.messages = append(h.messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: text,
	})

	// Construct the OpenAI request
	if model == "" {
		model = "qwen-plus"
	}

	request := openai.ChatCompletionRequest{
		Model:       model,
		Messages:    h.messages,
		Temperature: 0.7,
		Stream:      false,
	}

	// Create chat completion
	response, err := h.client.CreateChatCompletion(h.ctx, request)
	if err != nil {
		return "", fmt.Errorf("error creating chat completion: %w", err)
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no response choices returned")
	}

	content := response.Choices[0].Message.Content

	// Add assistant response to history
	h.messages = append(h.messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleAssistant,
		Content: content,
	})

	h.logger.WithField("response", content).Info("LLM query completed")

	return content, nil
}

// Reset clears the conversation history but keeps the system prompt
func (h *LLMHandler) Reset() {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	// Reset to just the system message
	h.messages = []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: h.systemMsg,
		},
	}
}

// GetMessages returns the current conversation messages
func (h *LLMHandler) GetMessages() []openai.ChatCompletionMessage {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	// Return a copy to avoid race conditions
	messages := make([]openai.ChatCompletionMessage, len(h.messages))
	copy(messages, h.messages)
	return messages
}

// SetSystemPrompt updates the system prompt
func (h *LLMHandler) SetSystemPrompt(prompt string) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	h.systemMsg = prompt
	// Update the first message if it's a system message
	if len(h.messages) > 0 && h.messages[0].Role == openai.ChatMessageRoleSystem {
		h.messages[0].Content = prompt
	}
}
