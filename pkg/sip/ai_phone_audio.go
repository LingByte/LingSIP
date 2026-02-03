package sip1

import (
	"fmt"
	"net"
	"time"

	"github.com/LingByte/LingSIP/pkg/logger"
	"github.com/pion/rtp"
	"go.uber.org/zap"
)

// playTTSAudio 播放TTS音频（阻塞式）
func (engine *AIPhoneEngine) playTTSAudio(session *ScriptSession, text, speakerID string) error {
	if text == "" {
		return nil
	}

	logger.Info("Playing TTS audio",
		zap.String("call_id", session.CallID),
		zap.String("text", text),
		zap.String("speaker_id", speakerID))

	// 调用TTS服务生成音频
	audioData, err := engine.callTTSService(text, speakerID)
	if err != nil {
		return fmt.Errorf("TTS service failed: %w", err)
	}

	// 播放音频
	return engine.playAudioBlocking(session.ClientAddr, audioData)
}

// playAudioBlocking 阻塞式音频播放
func (engine *AIPhoneEngine) playAudioBlocking(clientAddr string, audioData []int16) error {
	if len(audioData) == 0 {
		return nil
	}

	addr, err := net.ResolveUDPAddr("udp", clientAddr)
	if err != nil {
		return fmt.Errorf("failed to resolve client address: %w", err)
	}

	// 每20ms发送一个RTP包（160个样本，8000Hz采样率）
	samplesPerPacket := 160
	sequenceNumber := uint16(1)
	timestamp := uint32(0)
	ssrc := uint32(12345) // 固定SSRC

	for i := 0; i < len(audioData); i += samplesPerPacket {
		end := i + samplesPerPacket
		if end > len(audioData) {
			end = len(audioData)
		}

		// 转换PCM为μ-law
		mulawData := make([]byte, end-i)
		for j, sample := range audioData[i:end] {
			mulawData[j] = linearToMulaw(sample)
		}

		// 创建RTP包
		packet := &rtp.Packet{
			Header: rtp.Header{
				Version:        2,
				Padding:        false,
				Extension:      false,
				Marker:         false,
				PayloadType:    0, // PCMU
				SequenceNumber: sequenceNumber,
				Timestamp:      timestamp,
				SSRC:           ssrc,
			},
			Payload: mulawData,
		}

		// 序列化RTP包
		data, err := packet.Marshal()
		if err != nil {
			logger.Error("Failed to marshal RTP packet", zap.Error(err))
			continue
		}

		// 发送RTP包
		_, err = engine.server.rtpConn.WriteToUDP(data, addr)
		if err != nil {
			logger.Error("Failed to send RTP packet", zap.Error(err))
			continue
		}

		// 更新序列号和时间戳
		sequenceNumber++
		timestamp += uint32(len(mulawData))

		// 等待20ms（模拟实时播放）
		time.Sleep(20 * time.Millisecond)
	}

	logger.Debug("Audio playback completed",
		zap.String("client_addr", clientAddr),
		zap.Int("samples", len(audioData)))

	return nil
}

// listenForUserInput 监听用户语音输入
func (engine *AIPhoneEngine) listenForUserInput(session *ScriptSession, timeout time.Duration) (string, error) {
	logger.Info("Listening for user input",
		zap.String("call_id", session.CallID),
		zap.Duration("timeout", timeout))

	// 清空音频缓冲区
	session.audioBuffer = session.audioBuffer[:0]
	session.isListening = true

	defer func() {
		session.isListening = false
	}()

	// 解析客户端地址
	clientAddr, err := net.ResolveUDPAddr("udp", session.ClientAddr)
	if err != nil {
		return "", fmt.Errorf("failed to resolve client address: %w", err)
	}

	// 音频收集
	buffer := make([]byte, 1500)
	silenceCount := 0
	maxSilence := int(timeout.Milliseconds() / 20) // 20ms per packet
	minAudioPackets := 10                          // 最少需要10个音频包才认为有有效输入
	audioPacketCount := 0

	startTime := time.Now()

	for time.Since(startTime) < timeout {
		// 设置读取超时
		engine.server.rtpConn.SetReadDeadline(time.Now().Add(20 * time.Millisecond))

		n, receivedAddr, err := engine.server.rtpConn.ReadFromUDP(buffer)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				silenceCount++
				if silenceCount > maxSilence {
					break // 静音超时
				}
				continue
			}
			logger.Error("Failed to read RTP data", zap.Error(err))
			continue
		}

		// 检查是否来自目标客户端
		if !receivedAddr.IP.Equal(clientAddr.IP) {
			continue
		}

		// 解析RTP包
		packet := &rtp.Packet{}
		if err := packet.Unmarshal(buffer[:n]); err != nil {
			logger.Debug("Failed to parse RTP packet", zap.Error(err))
			continue
		}

		// 只处理PCMU (payload type 0)
		if packet.PayloadType != 0 {
			continue
		}

		audioPacketCount++
		silenceCount = 0 // 重置静音计数

		// 解码μ-law为PCM
		for _, mulawByte := range packet.Payload {
			pcm := mulawToLinear(mulawByte)
			session.audioBuffer = append(session.audioBuffer, pcm)
		}

		// 检查是否有足够的音频数据进行语音检测
		if audioPacketCount >= minAudioPackets && engine.detectSpeechEnd(session.audioBuffer) {
			logger.Info("Speech end detected", zap.String("call_id", session.CallID))
			break
		}
	}

	// 清除读取超时
	engine.server.rtpConn.SetReadDeadline(time.Time{})

	if len(session.audioBuffer) < 1600 { // 少于200ms的音频认为无效
		logger.Info("No valid audio input received", zap.String("call_id", session.CallID))
		return "", nil
	}

	logger.Info("Audio input collected",
		zap.String("call_id", session.CallID),
		zap.Int("samples", len(session.audioBuffer)),
		zap.Int("packets", audioPacketCount))

	// 调用ASR服务识别语音
	return engine.callASRService(session.audioBuffer)
}

// detectSpeechEnd 检测语音结束（简单的静音检测）
func (engine *AIPhoneEngine) detectSpeechEnd(audioData []int16) bool {
	if len(audioData) < 1600 { // 少于200ms
		return false
	}

	// 检查最后200ms是否为静音
	silenceThreshold := int16(500) // 静音阈值
	silenceSamples := 1600         // 200ms @ 8000Hz

	if len(audioData) < silenceSamples {
		return false
	}

	// 检查最后200ms的音频
	start := len(audioData) - silenceSamples
	for i := start; i < len(audioData); i++ {
		if audioData[i] > silenceThreshold || audioData[i] < -silenceThreshold {
			return false // 发现非静音
		}
	}

	return true // 检测到静音结束
}

// callTTSService 调用TTS服务
func (engine *AIPhoneEngine) callTTSService(text, speakerID string) ([]int16, error) {
	// 这里需要根据你的TTS服务接口实现
	// 示例实现：

	logger.Debug("Calling TTS service",
		zap.String("text", text),
		zap.String("speaker_id", speakerID))

	// TODO: 实现实际的TTS服务调用
	// 这里返回模拟的音频数据

	// 生成简单的测试音频（正弦波）
	sampleRate := 8000
	duration := len(text) * 100 // 每个字符100ms
	samples := sampleRate * duration / 1000

	audioData := make([]int16, samples)

	for i := 0; i < samples; i++ {
		// 生成简单的测试音频（降低音量避免过响）
		sample := 0.1 * float64(0x7FFF) * 0.1 // 很低的音量
		audioData[i] = int16(sample)
	}

	return audioData, nil
}

// callASRService 调用ASR服务
func (engine *AIPhoneEngine) callASRService(audioData []int16) (string, error) {
	logger.Debug("Calling ASR service", zap.Int("samples", len(audioData)))

	// TODO: 实现实际的ASR服务调用
	// 这里返回模拟的识别结果

	if len(audioData) < 800 { // 少于100ms认为无效
		return "", nil
	}

	// 模拟ASR识别结果
	mockResults := []string{
		"你好",
		"我需要帮助",
		"是的",
		"不是",
		"谢谢",
		"再见",
		"我想了解一下",
		"可以的",
		"没问题",
	}

	// 根据音频长度选择不同的结果
	index := (len(audioData) / 1000) % len(mockResults)
	result := mockResults[index]

	logger.Info("ASR result", zap.String("text", result))
	return result, nil
}

// callAIService 调用AI服务
func (engine *AIPhoneEngine) callAIService(session *ScriptSession, prompt string) (string, error) {
	logger.Debug("Calling AI service",
		zap.String("call_id", session.CallID),
		zap.String("prompt", prompt))

	// TODO: 实现实际的AI服务调用
	// 这里返回模拟的AI回复

	// 构建对话上下文
	messages := []map[string]string{
		{"role": "system", "content": prompt},
	}

	// 添加历史对话
	for _, msg := range session.Conversation {
		messages = append(messages, map[string]string{
			"role":    msg.Role,
			"content": msg.Content,
		})
	}

	// 模拟AI回复
	mockResponses := []string{
		"好的，我明白了。还有什么需要帮助的吗？",
		"谢谢您的回答。让我为您记录一下。",
		"请问您还有其他需要吗？",
		"我已经记录了您的信息，稍后会有专人联系您。",
		"感谢您的配合，祝您生活愉快！",
		"好的，我会为您安排相关服务。",
		"请保持电话畅通，我们会尽快联系您。",
	}

	// 根据对话轮数选择不同回复
	index := len(session.Conversation) % len(mockResponses)
	response := mockResponses[index]

	logger.Info("AI response",
		zap.String("call_id", session.CallID),
		zap.String("response", response))

	return response, nil
}

// evaluateCondition 评估条件表达式
func (engine *AIPhoneEngine) evaluateCondition(session *ScriptSession, condition string) (bool, error) {
	logger.Debug("Evaluating condition",
		zap.String("call_id", session.CallID),
		zap.String("condition", condition))

	// TODO: 实现条件表达式解析和评估
	// 这里实现简单的条件判断

	switch condition {
	case "has_job_need":
		// 检查对话中是否提到就业需求
		for _, msg := range session.Conversation {
			if msg.Role == "user" {
				content := msg.Content
				if contains(content, []string{"需要", "想要", "找工作", "就业", "培训"}) {
					return true, nil
				}
				if contains(content, []string{"不需要", "不用", "没有"}) {
					return false, nil
				}
			}
		}
		return false, nil

	case "user_satisfied":
		// 检查用户是否满意
		for _, msg := range session.Conversation {
			if msg.Role == "user" {
				content := msg.Content
				if contains(content, []string{"满意", "好的", "可以", "谢谢"}) {
					return true, nil
				}
				if contains(content, []string{"不满意", "不好", "不行"}) {
					return false, nil
				}
			}
		}
		return true, nil // 默认满意

	default:
		logger.Warn("Unknown condition", zap.String("condition", condition))
		return false, nil
	}
}

// shouldEndConversation 判断是否应该结束对话
func (engine *AIPhoneEngine) shouldEndConversation(aiResponse string) bool {
	endPhrases := []string{
		"再见",
		"祝您",
		"感谢您的配合",
		"通话结束",
		"拜拜",
	}

	return contains(aiResponse, endPhrases)
}

// contains 检查文本是否包含任一关键词
func contains(text string, keywords []string) bool {
	for _, keyword := range keywords {
		if len(keyword) > 0 && len(text) >= len(keyword) {
			for i := 0; i <= len(text)-len(keyword); i++ {
				if text[i:i+len(keyword)] == keyword {
					return true
				}
			}
		}
	}
	return false
}
