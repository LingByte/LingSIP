package llm

import (
	"fmt"

	"github.com/sirupsen/logrus"
)

// TTSAdapter adapts the SIP TTS functionality to work with LLM handler
type TTSAdapter struct {
	ttsFunc    func(text, speakerID string) ([]int16, error)
	playFunc   func(clientAddr string, audioData []int16) error
	hangupFunc func(reason string) error
	referFunc  func(caller, target string, headers map[string]string) error
	clientAddr string
	speakerID  string
	logger     *logrus.Logger
}

// NewTTSAdapter creates a new TTS adapter
func NewTTSAdapter(
	ttsFunc func(text, speakerID string) ([]int16, error),
	playFunc func(clientAddr string, audioData []int16) error,
	hangupFunc func(reason string) error,
	referFunc func(caller, target string, headers map[string]string) error,
	clientAddr string,
	speakerID string,
	logger *logrus.Logger,
) *TTSAdapter {
	return &TTSAdapter{
		ttsFunc:    ttsFunc,
		playFunc:   playFunc,
		hangupFunc: hangupFunc,
		referFunc:  referFunc,
		clientAddr: clientAddr,
		speakerID:  speakerID,
		logger:     logger,
	}
}

// TTS implements the TTSClient interface for regular TTS
func (a *TTSAdapter) TTS(text, voice, playID string, endOfStream, autoHangup bool, onStart, onEnd func(), interrupt bool) error {
	if text == "" {
		return nil
	}

	// Use the configured speaker ID if voice is not specified
	if voice == "" {
		voice = a.speakerID
	}

	a.logger.WithFields(logrus.Fields{
		"text":        text,
		"voice":       voice,
		"playID":      playID,
		"endOfStream": endOfStream,
		"autoHangup":  autoHangup,
	}).Debug("TTS request")

	// Call onStart callback if provided
	if onStart != nil {
		onStart()
	}

	// Generate audio using TTS
	audioData, err := a.ttsFunc(text, voice)
	if err != nil {
		return fmt.Errorf("TTS generation failed: %w", err)
	}

	// Play the audio
	if err := a.playFunc(a.clientAddr, audioData); err != nil {
		return fmt.Errorf("audio playback failed: %w", err)
	}

	// Call onEnd callback if provided
	if onEnd != nil {
		onEnd()
	}

	// Handle auto hangup if requested
	if autoHangup {
		if err := a.hangupFunc("Auto hangup after TTS"); err != nil {
			a.logger.WithError(err).Error("Failed to auto hangup")
		}
	}

	return nil
}

// StreamTTS implements the TTSClient interface for streaming TTS
func (a *TTSAdapter) StreamTTS(text, voice, playID string, endOfStream, autoHangup bool, onStart, onEnd func(), interrupt bool) error {
	// For now, we'll use the same implementation as regular TTS
	// In a real implementation, you might want to handle streaming differently
	return a.TTS(text, voice, playID, endOfStream, autoHangup, onStart, onEnd, interrupt)
}

// Hangup implements the TTSClient interface for hanging up calls
func (a *TTSAdapter) Hangup(reason string) error {
	a.logger.WithField("reason", reason).Info("Hanging up call")
	return a.hangupFunc(reason)
}

// Refer implements the TTSClient interface for call referral
func (a *TTSAdapter) Refer(caller, target string, headers map[string]string) error {
	a.logger.WithFields(logrus.Fields{
		"caller": caller,
		"target": target,
	}).Info("Referring call")
	return a.referFunc(caller, target, headers)
}

// SetClientAddr updates the client address
func (a *TTSAdapter) SetClientAddr(addr string) {
	a.clientAddr = addr
}

// SetSpeakerID updates the speaker ID
func (a *TTSAdapter) SetSpeakerID(speakerID string) {
	a.speakerID = speakerID
}
