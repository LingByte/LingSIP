package bootstrap

import (
	"errors"
	"fmt"

	"github.com/LingByte/LingSIP/internal/models"
	"github.com/LingByte/LingSIP/pkg/config"
	"github.com/LingByte/LingSIP/pkg/constants"
	"github.com/LingByte/LingSIP/pkg/logger"
	"github.com/LingByte/LingSIP/pkg/utils"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type SeedService struct {
	db *gorm.DB
}

func (s *SeedService) SeedAll() error {
	if err := s.seedConfigs(); err != nil {
		return err
	}
	if err := s.seedAIPhoneScripts(); err != nil {
		return err
	}
	return nil
}

func (s *SeedService) seedConfigs() error {
	defaults := []utils.Config{
		{Key: constants.KEY_SITE_URL, Desc: "Site URL", Autoload: true, Public: true, Format: "text", Value: func() string {
			if config.GlobalConfig.Server.URL != "" {
				return config.GlobalConfig.Server.URL
			}
			return "https://lingecho.com"
		}()},
		{Key: constants.KEY_SITE_NAME, Desc: "Site Name", Autoload: true, Public: true, Format: "text", Value: func() string {
			if config.GlobalConfig.Server.Name != "" {
				return config.GlobalConfig.Server.Name
			}
			return "LingEcho"
		}()},
		{Key: constants.KEY_SITE_LOGO_URL, Desc: "Site Logo", Autoload: true, Public: true, Format: "text", Value: func() string {
			if config.GlobalConfig.Server.Logo != "" {
				return config.GlobalConfig.Server.Logo
			}
			return "/static/img/favicon.png"
		}()},
		{Key: constants.KEY_SITE_DESCRIPTION, Desc: "Site Description", Autoload: true, Public: true, Format: "text", Value: func() string {
			if config.GlobalConfig.Server.Desc != "" {
				return config.GlobalConfig.Server.Desc
			}
			return "LingStorage - Intelligent Storage Service Platform"
		}()},
		{Key: constants.KEY_SITE_TERMS_URL, Desc: "Terms of Service", Autoload: true, Public: true, Format: "text", Value: func() string {
			if config.GlobalConfig.Server.TermsURL != "" {
				return config.GlobalConfig.Server.TermsURL
			}
			return "https://lingecho.com"
		}()},
	}
	for _, cfg := range defaults {
		var existingConfig utils.Config
		result := s.db.Where("`key` = ?", cfg.Key).First(&existingConfig)

		if result.Error != nil {
			if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
				return result.Error
			}
			if err := s.db.Create(&cfg).Error; err != nil {
				return err
			}
		} else {
			existingConfig.Value = cfg.Value
			existingConfig.Desc = cfg.Desc
			existingConfig.Autoload = cfg.Autoload
			existingConfig.Public = cfg.Public
			existingConfig.Format = cfg.Format
			if err := s.db.Save(&existingConfig).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

// seedAIPhoneScripts 初始化AI电话脚本数据
func (s *SeedService) seedAIPhoneScripts() error {
	// 检查是否已经存在示例脚本，避免重复插入
	var existingScript models.AIPhoneScript
	result := s.db.Where("name = ?", "就业需求调查").First(&existingScript)
	if result.Error == nil {
		logger.Info("AI phone script already exists, skipping seed")
		return nil
	}
	if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return fmt.Errorf("failed to check existing script: %w", result.Error)
	}

	// 创建就业调查脚本
	script := &models.AIPhoneScript{
		Name:        "就业需求调查",
		Description: "成都市金牛区就业局就业需求调查脚本",
		Version:     "1.0.0",
		Status:      models.ScriptStatusActive,
		SpeakerID:   "10001",
		StartStepID: "welcome",
		Category:    "government",
		Department:  "成都市金牛区就业局",
		Owner:       "system",
		MaxDuration: 300000, // 5分钟
		MaxSteps:    20,
	}

	if err := models.CreateAIPhoneScript(s.db, script); err != nil {
		return fmt.Errorf("failed to create sample script: %w", err)
	}

	// 创建步骤
	steps := []struct {
		ID   string
		Name string
		Type models.StepType
		Data models.StepData
	}{
		{
			ID:   "welcome",
			Name: "欢迎问候",
			Type: models.StepTypeCallout,
			Data: models.StepData{
				Prompt:    "你是成都市金牛区就业局的工作人员，需要调查市民的就业需求。请礼貌地询问对方是否有就业需要。",
				Welcome:   "你好，我是成都市金牛区就业局的工作人员",
				SpeakerID: "1",
				SliceTime: 30000,
				NextStep:  "check_need",
			},
		},
		{
			ID:   "check_need",
			Name: "检查就业需求",
			Type: models.StepTypeCondition,
			Data: models.StepData{
				Condition: "has_job_need",
				TrueNext:  "collect_need",
				FalseNext: "ending",
			},
		},
		{
			ID:   "collect_need",
			Name: "收集具体需求",
			Type: models.StepTypeCallout,
			Data: models.StepData{
				Prompt:    "用户有就业需求，请询问具体需要什么服务：找工作、就业培训还是创业服务？",
				SpeakerID: "1",
				SliceTime: 30000,
				NextStep:  "promise_contact",
			},
		},
		{
			ID:   "promise_contact",
			Name: "承诺联系",
			Type: models.StepTypePlayAudio,
			Data: models.StepData{
				AudioText: "请保持电话畅通，我们会尽快安排就业服务专员与您联系",
				SpeakerID: "1",
				NextStep:  "ending",
			},
		},
		{
			ID:   "ending",
			Name: "结束通话",
			Type: models.StepTypeCallout,
			Data: models.StepData{
				Prompt:    "告知用户如需任何服务，可前往居住地就近街道或社区便民服务中心，然后礼貌地道别结束对话。",
				SpeakerID: "1",
				SliceTime: 15000,
				NextStep:  "hangup",
			},
		},
		{
			ID:   "hangup",
			Name: "挂断电话",
			Type: models.StepTypeHangup,
			Data: models.StepData{},
		},
	}

	for i, stepConfig := range steps {
		step := &models.AIPhoneScriptStep{
			ScriptID: script.ID,
			StepID:   stepConfig.ID,
			GroupID:  "main",
			Name:     stepConfig.Name,
			Type:     stepConfig.Type,
			Order:    i,
			Data:     stepConfig.Data,
			Enabled:  true,
			Timeout:  30000,
		}

		if err := models.CreateScriptStep(s.db, step); err != nil {
			logger.Error("Failed to create sample step",
				zap.String("step_id", stepConfig.ID),
				zap.Error(err))
			continue
		}
	}

	// 创建电话号码映射
	phoneNumbers := []string{"10086", "95588", "400-123-4567"} // 多个示例号码
	for i, phoneNumber := range phoneNumbers {
		mapping := &models.ScriptPhoneMapping{
			ScriptID:    script.ID,
			PhoneNumber: phoneNumber,
			Priority:    i + 1,
			Enabled:     true,
			Description: fmt.Sprintf("就业调查热线 - %s", phoneNumber),
			StartTime:   "09:00:00",
			EndTime:     "18:00:00",
			WeekDays:    "1,2,3,4,5", // 工作日
		}

		if err := models.CreateScriptPhoneMapping(s.db, mapping); err != nil {
			logger.Error("Failed to create phone mapping",
				zap.String("phone", phoneNumber),
				zap.Error(err))
		}
	}

	logger.Info("AI phone script seeded successfully",
		zap.String("name", script.Name),
		zap.Int("steps", len(steps)),
		zap.Int("phone_numbers", len(phoneNumbers)))

	return nil
}
