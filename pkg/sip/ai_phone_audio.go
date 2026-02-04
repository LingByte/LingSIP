package sip1

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/LingByte/LingSIP/pkg/config"
	"github.com/LingByte/LingSIP/pkg/logger"
	"github.com/LingByte/LingSIP/pkg/recognizer"
	"github.com/LingByte/LingSIP/pkg/synthesizer"
	"github.com/LingByte/LingSIP/pkg/utils"
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

	// 清空音频缓冲区，确保对话隔离
	session.mutex.Lock()
	session.audioBuffer = make([]int16, 0, 96000) // 重新分配，确保完全清空
	session.isListening = true
	session.mutex.Unlock()

	defer func() {
		session.mutex.Lock()
		session.isListening = false
		session.mutex.Unlock()
	}()

	// 解析客户端地址
	clientAddr, err := net.ResolveUDPAddr("udp", session.ClientAddr)
	if err != nil {
		return "", fmt.Errorf("failed to resolve client address: %w", err)
	}

	// 音频收集参数
	buffer := make([]byte, 1500)

	// 等待阶段参数
	waitingForSpeech := true
	speechStartTime := time.Time{}
	noSpeechTimeout := 8 * time.Second // 等待用户开始说话的超时时间，增加到8秒

	// 语音检测参数
	minAudioPackets := 25  // 最少需要25个音频包才认为有有效输入（500ms）
	maxAudioPackets := 600 // 最多收集600个包（12秒音频）
	audioPacketCount := 0
	hasValidAudio := false
	consecutiveSilencePackets := 0
	maxConsecutiveSilence := 100 // 连续静音包数量阈值（2秒）

	// 音频质量检测参数
	silenceThreshold := int16(500) // 静音阈值，提高阈值避免背景噪音
	validAudioThreshold := 0.2     // 有效音频样本比例阈值，降低阈值

	startTime := time.Now()

	for time.Since(startTime) < timeout && audioPacketCount < maxAudioPackets {
		// 设置读取超时
		engine.server.rtpConn.SetReadDeadline(time.Now().Add(50 * time.Millisecond))

		n, receivedAddr, err := engine.server.rtpConn.ReadFromUDP(buffer)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				// 检查是否在等待用户开始说话阶段超时
				if waitingForSpeech && time.Since(startTime) > noSpeechTimeout {
					logger.Info("No speech detected within timeout",
						zap.String("call_id", session.CallID),
						zap.Duration("waited", time.Since(startTime)))
					return "", nil // 用户没有说话
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

		// 解码μ-law为PCM并检查音频质量
		validSamples := 0
		totalSamples := len(packet.Payload)
		packetSamples := make([]int16, totalSamples)

		session.mutex.Lock()
		for i, mulawByte := range packet.Payload {
			pcm := mulawToLinear(mulawByte)
			packetSamples[i] = pcm
			session.audioBuffer = append(session.audioBuffer, pcm)

			// 检查是否有有效音频信号（超过静音阈值）
			if pcm > silenceThreshold || pcm < -silenceThreshold {
				validSamples++
			}
		}
		session.mutex.Unlock()

		// 判断这个包是否包含有效音频
		validRatio := float64(validSamples) / float64(totalSamples)
		isValidPacket := validRatio > validAudioThreshold

		// 添加调试信息
		if audioPacketCount%50 == 0 { // 每50个包打印一次调试信息
			logger.Debug("Audio packet analysis",
				zap.String("call_id", session.CallID),
				zap.Int("packet_count", audioPacketCount),
				zap.Int("valid_samples", validSamples),
				zap.Int("total_samples", totalSamples),
				zap.Float64("valid_ratio", validRatio),
				zap.Bool("is_valid", isValidPacket),
				zap.Int("consecutive_silence", consecutiveSilencePackets))
		}

		if isValidPacket {
			if waitingForSpeech {
				// 检测到用户开始说话
				waitingForSpeech = false
				speechStartTime = time.Now()
				logger.Info("Speech detected, starting recording",
					zap.String("call_id", session.CallID),
					zap.Duration("wait_time", time.Since(startTime)))
			}
			hasValidAudio = true
			consecutiveSilencePackets = 0
		} else {
			consecutiveSilencePackets++
		}

		// 如果还在等待用户说话，继续等待
		if waitingForSpeech {
			continue
		}

		// 语音结束检测：已经有足够的音频且连续静音时间过长
		if audioPacketCount >= minAudioPackets && hasValidAudio &&
			consecutiveSilencePackets >= maxConsecutiveSilence {
			// 额外检查：确保已经录制了至少2秒的音频
			if !speechStartTime.IsZero() && time.Since(speechStartTime) >= 2*time.Second {
				logger.Info("Speech end detected by consecutive silence",
					zap.String("call_id", session.CallID),
					zap.Int("total_packets", audioPacketCount),
					zap.Int("silence_packets", consecutiveSilencePackets),
					zap.Duration("speech_duration", time.Since(speechStartTime)))
				break
			} else {
				// 如果录制时间不够，重置静音计数，继续录制
				consecutiveSilencePackets = maxConsecutiveSilence / 2 // 减少一半静音计数而不是完全重置
				logger.Debug("Speech too short, continuing recording",
					zap.String("call_id", session.CallID),
					zap.Duration("current_duration", time.Since(speechStartTime)))
			}
		}

		// 如果语音时间过长，也要结束
		if !speechStartTime.IsZero() && time.Since(speechStartTime) > 10*time.Second {
			logger.Info("Speech duration limit reached",
				zap.String("call_id", session.CallID),
				zap.Duration("duration", time.Since(speechStartTime)))
			break
		}
	}

	// 清除读取超时
	engine.server.rtpConn.SetReadDeadline(time.Time{})

	// 如果还在等待用户说话阶段，说明用户没有回应
	if waitingForSpeech {
		logger.Info("User did not respond within timeout",
			zap.String("call_id", session.CallID),
			zap.Duration("total_wait", time.Since(startTime)))
		return "", nil
	}

	// 获取最终的音频数据（线程安全）
	session.mutex.Lock()
	audioData := make([]int16, len(session.audioBuffer))
	copy(audioData, session.audioBuffer)
	session.mutex.Unlock()

	// 检查音频长度和质量
	audioLengthMs := len(audioData) * 1000 / 8000 // 8kHz采样率

	if len(audioData) < 8000 { // 少于1秒的音频认为无效
		logger.Info("Audio too short, considered invalid",
			zap.String("call_id", session.CallID),
			zap.Int("samples", len(audioData)),
			zap.Int("length_ms", audioLengthMs),
			zap.Int("packets", audioPacketCount),
			zap.Bool("has_valid_audio", hasValidAudio))
		return "", nil
	}

	logger.Info("Audio input collected successfully",
		zap.String("call_id", session.CallID),
		zap.Int("samples", len(audioData)),
		zap.Int("length_ms", audioLengthMs),
		zap.Int("packets", audioPacketCount),
		zap.Bool("has_valid_audio", hasValidAudio),
		zap.Duration("speech_duration", time.Since(speechStartTime)))

	// 调用ASR服务识别语音
	return engine.callASRService(audioData)
}

// listenForDTMF 监听DTMF按键输入
func (engine *AIPhoneEngine) listenForDTMF(session *ScriptSession, timeout time.Duration, maxDigits int, terminator string) (string, error) {
	logger.Info("Listening for DTMF input",
		zap.String("call_id", session.CallID),
		zap.Duration("timeout", timeout),
		zap.Int("max_digits", maxDigits),
		zap.String("terminator", terminator))

	// 解析客户端地址
	clientAddr, err := net.ResolveUDPAddr("udp", session.ClientAddr)
	if err != nil {
		return "", fmt.Errorf("failed to resolve client address: %w", err)
	}

	// DTMF检测参数
	buffer := make([]byte, 1500)
	dtmfInput := ""
	startTime := time.Now()

	// DTMF频率检测表（简化版本）
	dtmfFreqs := map[string]string{
		"1": "697,1209", "2": "697,1336", "3": "697,1477", "A": "697,1633",
		"4": "770,1209", "5": "770,1336", "6": "770,1477", "B": "770,1633",
		"7": "852,1209", "8": "852,1336", "9": "852,1477", "C": "852,1633",
		"*": "941,1209", "0": "941,1336", "#": "941,1477", "D": "941,1633",
	}

	for time.Since(startTime) < timeout && len(dtmfInput) < maxDigits {
		// 设置读取超时
		engine.server.rtpConn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))

		n, receivedAddr, err := engine.server.rtpConn.ReadFromUDP(buffer)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			logger.Error("Failed to read RTP data for DTMF", zap.Error(err))
			continue
		}

		// 检查是否来自目标客户端
		if !receivedAddr.IP.Equal(clientAddr.IP) {
			continue
		}

		// 解析RTP包
		packet := &rtp.Packet{}
		if err := packet.Unmarshal(buffer[:n]); err != nil {
			continue
		}

		// 检查是否是DTMF事件包 (payload type 101)
		if packet.PayloadType == 101 {
			// 解析DTMF事件
			if len(packet.Payload) >= 4 {
				event := packet.Payload[0]
				// end := (packet.Payload[1] & 0x80) != 0
				// volume := packet.Payload[1] & 0x3F
				// duration := binary.BigEndian.Uint16(packet.Payload[2:4])

				// DTMF事件映射
				dtmfMap := map[byte]string{
					0: "0", 1: "1", 2: "2", 3: "3", 4: "4", 5: "5", 6: "6", 7: "7", 8: "8", 9: "9",
					10: "*", 11: "#", 12: "A", 13: "B", 14: "C", 15: "D",
				}

				if digit, exists := dtmfMap[event]; exists {
					dtmfInput += digit
					logger.Info("DTMF digit detected",
						zap.String("call_id", session.CallID),
						zap.String("digit", digit),
						zap.String("current_input", dtmfInput))

					// 检查是否遇到终止符
					if digit == terminator {
						// 移除终止符
						if len(dtmfInput) > 0 {
							dtmfInput = dtmfInput[:len(dtmfInput)-1]
						}
						break
					}
				}
			}
		} else if packet.PayloadType == 0 {
			// 如果没有专门的DTMF事件，尝试从音频中检测DTMF（简化实现）
			// 这里可以实现基于频率分析的DTMF检测，但比较复杂
			// 暂时跳过音频DTMF检测
			continue
		}
	}

	// 清除读取超时
	engine.server.rtpConn.SetReadDeadline(time.Time{})

	if dtmfInput == "" {
		logger.Info("No DTMF input detected within timeout",
			zap.String("call_id", session.CallID),
			zap.Duration("waited", time.Since(startTime)))
		return "", nil
	}

	logger.Info("DTMF input completed",
		zap.String("call_id", session.CallID),
		zap.String("input", dtmfInput),
		zap.Duration("duration", time.Since(startTime)))

	// 避免编译器警告
	_ = dtmfFreqs

	return dtmfInput, nil
}

// detectSpeechEnd 检测语音结束（简化版本，主要逻辑已移到listenForUserInput中）
func (engine *AIPhoneEngine) detectSpeechEnd(audioData []int16) bool {
	// 这个函数现在主要用作备用检测
	if len(audioData) < 2400 { // 少于300ms
		return false
	}

	// 检查最后300ms是否为静音
	silenceThreshold := int16(200)
	silenceSamples := 2400 // 300ms @ 8000Hz

	if len(audioData) < silenceSamples {
		return false
	}

	// 检查最后300ms的音频
	start := len(audioData) - silenceSamples
	silentSamples := 0

	for i := start; i < len(audioData); i++ {
		if audioData[i] <= silenceThreshold && audioData[i] >= -silenceThreshold {
			silentSamples++
		}
	}

	// 如果85%以上的样本都是静音，认为语音结束
	silenceRatio := float64(silentSamples) / float64(silenceSamples)
	return silenceRatio > 0.85
}

// callTTSService 调用TTS服务
func (engine *AIPhoneEngine) callTTSService(text, speakerID string) ([]int16, error) {
	logger.Debug("Calling TTS service",
		zap.String("text", text),
		zap.String("speaker_id", speakerID))

	// 从全局配置获取TTS配置
	ttsConfig := config.GlobalConfig.Services.TTS

	// 创建TTS配置
	var ttsCredentialConfig synthesizer.TTSCredentialConfig

	switch ttsConfig.Provider {
	case "qcloud", "tencent":
		ttsCredentialConfig = synthesizer.TTSCredentialConfig{
			"provider":   "tencent",
			"appId":      ttsConfig.AppID,
			"secretId":   ttsConfig.SecretID,
			"secretKey":  ttsConfig.SecretKey,
			"voiceType":  ttsConfig.VoiceType,
			"sampleRate": ttsConfig.SampleRate,
			"codec":      ttsConfig.Codec,
		}
	case "baidu":
		ttsCredentialConfig = synthesizer.TTSCredentialConfig{
			"provider":   "baidu",
			"appId":      ttsConfig.AppID,
			"apiKey":     ttsConfig.SecretID,
			"secretKey":  ttsConfig.SecretKey,
			"voiceType":  ttsConfig.VoiceType,
			"sampleRate": ttsConfig.SampleRate,
			"codec":      ttsConfig.Codec,
		}
	case "aws":
		ttsCredentialConfig = synthesizer.TTSCredentialConfig{
			"provider":     "aws",
			"accessKey":    ttsConfig.SecretID,
			"secretKey":    ttsConfig.SecretKey,
			"region":       ttsConfig.Region,
			"voiceId":      ttsConfig.VoiceType,
			"outputFormat": ttsConfig.Codec,
			"sampleRate":   ttsConfig.SampleRate,
		}
	default:
		// 默认使用腾讯云配置（向后兼容）
		ttsCredentialConfig = synthesizer.TTSCredentialConfig{
			"provider":   "tencent",
			"appId":      utils.GetEnv("TTS_APP_ID"),
			"secretId":   utils.GetEnv("TTS_SECRET_ID"),
			"secretKey":  utils.GetEnv("TTS_SECRET_KEY"),
			"voiceType":  utils.GetEnv("TTS_VOICE_TYPE"),
			"sampleRate": utils.GetIntEnv("TTS_SAMPLE_RATE"),
			"codec":      utils.GetEnv("TTS_CODEC"),
		}

		// 如果环境变量为空，使用默认值
		if ttsCredentialConfig["appId"] == "" {
			ttsCredentialConfig["appId"] = ""
		}
		if ttsCredentialConfig["secretId"] == "" {
			ttsCredentialConfig["secretId"] = ""
		}
		if ttsCredentialConfig["secretKey"] == "" {
			ttsCredentialConfig["secretKey"] = ""
		}
		if ttsCredentialConfig["voiceType"] == "" {
			ttsCredentialConfig["voiceType"] = ""
		}
		if ttsCredentialConfig["sampleRate"] == int64(0) {
			ttsCredentialConfig["sampleRate"] = 8000
		}
		if ttsCredentialConfig["codec"] == "" {
			ttsCredentialConfig["codec"] = "pcm"
		}
	}

	// 创建TTS服务
	ttsService, err := synthesizer.NewSynthesisServiceFromCredential(ttsCredentialConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create TTS service: %w", err)
	}
	defer ttsService.Close()

	// 创建音频缓冲区
	buffer := &synthesizer.SynthesisBuffer{}

	// 调用TTS合成
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = ttsService.Synthesize(ctx, buffer, text)
	if err != nil {
		return nil, fmt.Errorf("TTS synthesis failed: %w", err)
	}

	if len(buffer.Data) == 0 {
		return nil, fmt.Errorf("TTS returned empty audio data")
	}

	// 将字节数据转换为PCM样本
	audioData := make([]int16, len(buffer.Data)/2)
	for i := 0; i < len(audioData); i++ {
		// 小端字节序转换
		audioData[i] = int16(buffer.Data[i*2]) | int16(buffer.Data[i*2+1])<<8
	}

	// 音频质量检查和调整
	if len(audioData) > 0 {
		// 检查音频幅度，如果太小则放大
		maxAmplitude := int16(0)
		for _, sample := range audioData {
			if sample < 0 {
				sample = -sample
			}
			if sample > maxAmplitude {
				maxAmplitude = sample
			}
		}

		// 如果音频太小，适当放大（但不要过度）
		if maxAmplitude < 8000 && maxAmplitude > 0 {
			amplifyRatio := float64(8000) / float64(maxAmplitude)
			if amplifyRatio > 4.0 {
				amplifyRatio = 4.0 // 最多放大4倍
			}

			for i := range audioData {
				audioData[i] = int16(float64(audioData[i]) * amplifyRatio)
			}

			logger.Debug("Audio amplified",
				zap.Float64("ratio", amplifyRatio),
				zap.Int16("original_max", maxAmplitude))
		}
	}

	logger.Info("TTS synthesis completed",
		zap.String("provider", ttsConfig.Provider),
		zap.String("text", text),
		zap.Int("samples", len(audioData)))

	return audioData, nil
}

// callASRService 调用ASR服务
func (engine *AIPhoneEngine) callASRService(audioData []int16) (string, error) {
	logger.Debug("Calling ASR service", zap.Int("samples", len(audioData)))

	if len(audioData) < 8000 { // 少于1秒认为无效
		logger.Info("Audio too short for ASR", zap.Int("samples", len(audioData)))
		return "", nil
	}

	// 限制音频长度，避免过长的音频影响识别效果
	maxSamples := 80000 // 最多10秒音频 (10 * 8000)
	if len(audioData) > maxSamples {
		logger.Debug("Truncating audio data",
			zap.Int("original_samples", len(audioData)),
			zap.Int("truncated_samples", maxSamples))
		audioData = audioData[:maxSamples]
	}

	// 从全局配置获取ASR配置
	asrConfig := config.GlobalConfig.Services.ASR

	var asr recognizer.TranscribeService
	var err error

	switch asrConfig.Provider {
	case "qcloud", "tencent":
		// 创建腾讯云ASR配置
		qcloudConfig := recognizer.NewQcloudASROption(
			asrConfig.AppID,
			asrConfig.SecretID,
			asrConfig.SecretKey,
		)
		qcloudConfig.ModelType = asrConfig.ModelType
		if qcloudConfig.ModelType == "" {
			qcloudConfig.ModelType = "8k_zh" // 默认8k中文模型，适合电话音质
		}
		asr = recognizer.NewQcloudASR(qcloudConfig)

	case "google":
		// 创建Google ASR配置
		googleConfig := recognizer.GoogleASROption{
			LanguageCode: asrConfig.Language,
		}
		if googleConfig.LanguageCode == "" {
			googleConfig.LanguageCode = "zh-CN"
		}
		googleASR := recognizer.NewGoogleASR(googleConfig)
		asr = &googleASR

	case "qiniu":
		// 创建七牛云ASR配置
		qiniuConfig := recognizer.QiniuASROption{
			APIKey: asrConfig.SecretID,
		}
		asr = recognizer.NewQiniuASR(qiniuConfig)

	default:
		// 默认使用腾讯云配置（向后兼容）
		appID := utils.GetEnv("ASR_APP_ID")
		secretID := utils.GetEnv("ASR_SECRET_ID")
		secretKey := utils.GetEnv("ASR_SECRET_KEY")

		// 如果环境变量为空，使用默认值
		if appID == "" {
			appID = ""
		}
		if secretID == "" {
			secretID = ""
		}
		if secretKey == "" {
			secretKey = ""
		}

		qcloudConfig := recognizer.NewQcloudASROption(appID, secretID, secretKey)
		qcloudConfig.ModelType = "8k_zh" // 8k中文模型，适合电话音质
		asr = recognizer.NewQcloudASR(qcloudConfig)
	}

	// 设置结果回调
	var result string
	var asrError error
	done := make(chan bool, 1)

	asr.Init(
		func(text string, isFinal bool, duration time.Duration, dialogID string) {
			logger.Debug("ASR callback received",
				zap.String("text", text),
				zap.Bool("is_final", isFinal),
				zap.Duration("duration", duration))

			if text != "" {
				result = text
				if isFinal {
					logger.Info("ASR recognition completed",
						zap.String("provider", asrConfig.Provider),
						zap.String("text", text),
						zap.Duration("duration", duration))
					done <- true
				}
			} else if isFinal {
				// 即使没有文本，如果是最终结果也要结束
				logger.Info("ASR recognition completed with empty result")
				done <- true
			}
		},
		func(err error, isFinal bool) {
			logger.Error("ASR error callback",
				zap.String("provider", asrConfig.Provider),
				zap.Error(err),
				zap.Bool("is_final", isFinal))
			if isFinal {
				asrError = err
				done <- true
			}
		},
	)

	// 启动ASR连接
	dialogID := fmt.Sprintf("dialog_%d", time.Now().UnixNano())
	err = asr.ConnAndReceive(dialogID)
	if err != nil {
		return "", fmt.Errorf("failed to connect ASR: %w", err)
	}
	defer asr.StopConn()

	// 将PCM样本转换为字节数据
	audioBytes := make([]byte, len(audioData)*2)
	for i, sample := range audioData {
		// 小端字节序
		audioBytes[i*2] = byte(sample & 0xFF)
		audioBytes[i*2+1] = byte((sample >> 8) & 0xFF)
	}

	logger.Debug("Sending audio data to ASR",
		zap.String("provider", asrConfig.Provider),
		zap.Int("audio_bytes", len(audioBytes)),
		zap.Int("samples", len(audioData)),
		zap.Int("duration_ms", len(audioData)*1000/8000))

	// 分块发送音频数据（每次发送1600字节，约100ms的音频）
	chunkSize := 1600
	for i := 0; i < len(audioBytes); i += chunkSize {
		end := i + chunkSize
		if end > len(audioBytes) {
			end = len(audioBytes)
		}

		chunk := audioBytes[i:end]
		err = asr.SendAudioBytes(chunk)
		if err != nil {
			return "", fmt.Errorf("failed to send audio chunk: %w", err)
		}

		// 稍微延迟模拟实时发送
		time.Sleep(50 * time.Millisecond)
	}

	// 发送结束标志
	err = asr.SendEnd()
	if err != nil {
		return "", fmt.Errorf("failed to send end signal: %w", err)
	}

	logger.Debug("Audio data sent, waiting for ASR result")

	// 等待识别结果（最多等待15秒）
	select {
	case <-done:
		if asrError != nil {
			return "", asrError
		}
		if result == "" {
			logger.Info("ASR returned empty result")
			return "", nil
		}
		return result, nil
	case <-time.After(15 * time.Second):
		return "", fmt.Errorf("ASR recognition timeout")
	}
}

// callAIService 调用AI服务
func (engine *AIPhoneEngine) callAIService(session *ScriptSession, prompt string) (string, error) {
	logger.Debug("Calling AI service",
		zap.String("call_id", session.CallID),
		zap.String("prompt", prompt))

	// 如果有LLM服务，使用LLM服务
	if engine.llmService != nil {
		// 构建完整的提示词，包含上下文
		fullPrompt := engine.buildPromptWithContext(session, prompt)

		response, err := engine.llmService.Query(fullPrompt)
		if err != nil {
			logger.Error("LLM service call failed",
				zap.String("call_id", session.CallID),
				zap.Error(err))
			// 降级到模拟回复
			return engine.getMockAIResponse(session), nil
		}

		logger.Info("AI response from LLM",
			zap.String("call_id", session.CallID),
			zap.String("response", response))

		return response, nil
	}

	// 降级到模拟AI回复
	response := engine.getMockAIResponse(session)

	logger.Info("AI response (mock)",
		zap.String("call_id", session.CallID),
		zap.String("response", response))

	return response, nil
}

// buildPromptWithContext 构建包含上下文的提示词
func (engine *AIPhoneEngine) buildPromptWithContext(session *ScriptSession, basePrompt string) string {
	// 构建对话历史
	conversationHistory := ""
	for _, msg := range session.Conversation {
		role := "用户"
		if msg.Role == "assistant" {
			role = "助手"
		}
		conversationHistory += fmt.Sprintf("%s: %s\n", role, msg.Content)
	}

	// 构建完整提示词
	fullPrompt := fmt.Sprintf(`%s

对话历史:
%s

请根据以上对话历史和角色设定，生成合适的回复。回复要求：
1. 保持角色一致性
2. 语言自然流畅
3. 回复简洁明了，适合电话对话
4. 如果用户表示不需要服务或要结束通话，请礼貌地结束对话

当前用户输入需要回复。`, basePrompt, conversationHistory)

	return fullPrompt
}

// getMockAIResponse 获取模拟AI回复（降级方案）
func (engine *AIPhoneEngine) getMockAIResponse(session *ScriptSession) string {
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
	return mockResponses[index]
}

// evaluateCondition 评估条件表达式
func (engine *AIPhoneEngine) evaluateCondition(session *ScriptSession, condition string) (bool, error) {
	logger.Debug("Evaluating condition",
		zap.String("call_id", session.CallID),
		zap.String("condition", condition))

	// 获取用户回应状态
	responseStatus := engine.getUserResponseStatus(session)
	isEngaged := engine.isUserEngaged(session)

	logger.Debug("User response status",
		zap.String("call_id", session.CallID),
		zap.String("status", responseStatus),
		zap.Bool("engaged", isEngaged))

	switch condition {
	case "has_job_need":
		// 如果用户没有回应，默认认为没有需求
		if !isEngaged {
			logger.Info("User not engaged, assuming no job need",
				zap.String("call_id", session.CallID),
				zap.String("response_status", responseStatus))
			return false, nil
		}

		// 检查对话中是否提到就业需求
		hasPositive := false
		hasNegative := false
		hasGreeting := false

		for _, msg := range session.Conversation {
			if msg.Role == "user" {
				content := msg.Content
				// 明确的就业需求关键词
				if contains(content, []string{"需要", "想要", "找工作", "就业", "培训", "创业", "失业", "工作", "招聘"}) {
					hasPositive = true
				}
				// 明确的拒绝关键词
				if contains(content, []string{"不需要", "不用", "没有", "不是", "不对", "不要", "没兴趣", "不找"}) {
					hasNegative = true
				}
				// 问候或确认关键词
				if contains(content, []string{"你好", "喂", "听到", "可以", "能听", "在吗", "什么事"}) {
					hasGreeting = true
				}
			}
		}

		// 如果有明确的就业需求，返回true
		if hasPositive && !hasNegative {
			logger.Info("Positive job need detected",
				zap.String("call_id", session.CallID))
			return true, nil
		}

		// 如果有明确的拒绝，返回false
		if hasNegative {
			logger.Info("Negative job need detected",
				zap.String("call_id", session.CallID))
			return false, nil
		}

		// 如果只是问候或确认，但没有明确表达需求，应该继续询问
		if hasGreeting && !hasPositive && !hasNegative {
			logger.Info("User greeting detected, need further inquiry",
				zap.String("call_id", session.CallID))
			// 这里我们可以设置一个标记，让脚本继续询问
			session.Context["needs_further_inquiry"] = true
			return false, nil // 暂时返回false，但标记需要进一步询问
		}

		// 如果有对话但不明确，默认返回false
		logger.Info("Ambiguous response, assuming no job need",
			zap.String("call_id", session.CallID))
		return false, nil

	case "needs_further_inquiry":
		// 检查是否需要进一步询问
		if needsInquiry, exists := session.Context["needs_further_inquiry"]; exists && needsInquiry.(bool) {
			return true, nil
		}
		return false, nil

	case "user_satisfied":
		// 检查用户是否满意
		for _, msg := range session.Conversation {
			if msg.Role == "user" {
				content := msg.Content
				if contains(content, []string{"满意", "好的", "可以", "谢谢", "行", "好"}) {
					return true, nil
				}
				if contains(content, []string{"不满意", "不好", "不行", "不可以", "不对"}) {
					return false, nil
				}
			}
		}
		return true, nil // 默认满意

	case "has_user_response":
		// 检查是否有用户回应
		return isEngaged, nil

	case "user_engaged":
		// 检查用户是否参与对话
		return isEngaged, nil

	case "collect_success":
		// 检查信息收集是否成功
		if collectFailed, exists := session.Context["collect_failed"]; exists && collectFailed.(bool) {
			return false, nil
		}
		return isEngaged, nil

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

// getUserResponseStatus 获取用户回应状态
func (engine *AIPhoneEngine) getUserResponseStatus(session *ScriptSession) string {
	if noResponse, exists := session.Context["no_user_response"]; exists && noResponse.(bool) {
		if retryCount, exists := session.Context["retry_count"]; exists {
			return fmt.Sprintf("no_response_after_%d_attempts", retryCount.(int))
		}
		return "no_response"
	}

	if collectFailed, exists := session.Context["collect_failed"]; exists && collectFailed.(bool) {
		return "collect_failed"
	}

	if len(session.Conversation) > 0 {
		return "has_response"
	}

	return "unknown"
}

// isUserEngaged 判断用户是否参与对话
func (engine *AIPhoneEngine) isUserEngaged(session *ScriptSession) bool {
	// 检查是否有有效的用户回应
	userMessageCount := 0
	for _, msg := range session.Conversation {
		if msg.Role == "user" && len(msg.Content) > 0 {
			userMessageCount++
		}
	}

	// 如果有用户消息且没有标记为无回应，认为用户参与了
	if userMessageCount > 0 {
		if noResponse, exists := session.Context["no_user_response"]; !exists || !noResponse.(bool) {
			return true
		}
	}

	return false
}
