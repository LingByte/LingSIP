package sip1

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/LingByte/LingSIP/internal/models"
	"github.com/LingByte/LingSIP/pkg/logger"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ScriptManager 脚本管理器
type ScriptManager struct {
	db    *gorm.DB
	cache map[string]*models.AIPhoneScript
	mutex sync.RWMutex
}

// NewScriptManager 创建脚本管理器
func NewScriptManager(db *gorm.DB) *ScriptManager {
	return &ScriptManager{
		db:    db,
		cache: make(map[string]*models.AIPhoneScript),
	}
}

// LoadScriptFromJSON 从JSON文件加载脚本
func (sm *ScriptManager) LoadScriptFromJSON(jsonPath string) error {
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return fmt.Errorf("failed to read script file: %w", err)
	}

	// 解析JSON脚本配置
	var scriptConfig struct {
		Name      string `json:"name"`
		SpeakerID string `json:"speakerId"`
		StartID   string `json:"startId"`
		Groups    []struct {
			ID    string `json:"id"`
			Name  string `json:"name"`
			Steps []struct {
				ID   string                 `json:"id"`
				Type string                 `json:"type"`
				Data map[string]interface{} `json:"data"`
			} `json:"steps"`
		} `json:"groups"`
	}

	if err := json.Unmarshal(data, &scriptConfig); err != nil {
		return fmt.Errorf("failed to parse script JSON: %w", err)
	}

	// 创建脚本记录
	script := &models.AIPhoneScript{
		Name:        scriptConfig.Name,
		Description: fmt.Sprintf("Loaded from %s", filepath.Base(jsonPath)),
		Version:     "1.0.0",
		Status:      models.ScriptStatusActive,
		SpeakerID:   scriptConfig.SpeakerID,
		StartStepID: scriptConfig.StartID,
		Category:    "imported",
		MaxDuration: 300000, // 5分钟
		MaxSteps:    50,
	}

	// 保存脚本到数据库
	if err := models.CreateAIPhoneScript(sm.db, script); err != nil {
		return fmt.Errorf("failed to create script: %w", err)
	}

	// 创建步骤记录
	stepOrder := 0
	for _, group := range scriptConfig.Groups {
		for _, stepConfig := range group.Steps {
			// 转换步骤数据
			stepData := models.StepData{}

			// 根据步骤类型转换数据
			switch stepConfig.Type {
			case "callout":
				if prompt, ok := stepConfig.Data["prompt"].(string); ok {
					stepData.Prompt = prompt
				}
				if welcome, ok := stepConfig.Data["welcome"].(string); ok {
					stepData.Welcome = welcome
				}
				if speakerID, ok := stepConfig.Data["speakerId"].(string); ok {
					stepData.SpeakerID = speakerID
				}
				if sliceTime, ok := stepConfig.Data["sliceTime"].(float64); ok {
					stepData.SliceTime = int(sliceTime)
				}
				if nextStep, ok := stepConfig.Data["nextStep"].(string); ok {
					stepData.NextStep = nextStep
				}
			}

			step := &models.AIPhoneScriptStep{
				ScriptID: script.ID,
				StepID:   stepConfig.ID,
				GroupID:  group.ID,
				Name:     fmt.Sprintf("Step %s", stepConfig.ID),
				Type:     models.StepType(stepConfig.Type),
				Order:    stepOrder,
				Data:     stepData,
				Enabled:  true,
				Timeout:  30000,
			}

			if err := models.CreateScriptStep(sm.db, step); err != nil {
				logger.Error("Failed to create script step",
					zap.String("step_id", stepConfig.ID),
					zap.Error(err))
				continue
			}

			stepOrder++
		}
	}

	// 重新加载脚本（包含步骤）
	loadedScript, err := models.GetAIPhoneScriptByID(sm.db, script.ID)
	if err != nil {
		return fmt.Errorf("failed to reload script: %w", err)
	}

	// 更新缓存
	sm.mutex.Lock()
	sm.cache[script.Name] = loadedScript
	sm.mutex.Unlock()

	logger.Info("Script loaded successfully",
		zap.String("name", script.Name),
		zap.String("file", jsonPath),
		zap.Int("steps", len(loadedScript.Steps)))

	return nil
}

// RefreshCache 刷新缓存
func (sm *ScriptManager) RefreshCache() error {
	scripts, err := models.GetActiveAIPhoneScripts(sm.db)
	if err != nil {
		return err
	}

	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	// 清空缓存
	sm.cache = make(map[string]*models.AIPhoneScript)

	// 重新加载
	for _, script := range scripts {
		fullScript, err := models.GetAIPhoneScriptByID(sm.db, script.ID)
		if err != nil {
			logger.Error("Failed to load script",
				zap.String("name", script.Name),
				zap.Error(err))
			continue
		}
		sm.cache[script.Name] = fullScript
	}

	logger.Info("Script cache refreshed", zap.Int("count", len(sm.cache)))
	return nil
}
