// config.go 定义 agent 示例的配置结构体与加载逻辑（从 config.json 读取）。
package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config agent 示例配置。
type Config struct {
	BotId            string   `json:"bot_id"`                       // 企业微信机器人 ID
	BotSecret        string   `json:"bot_secret"`                   // 机器人 Secret
	AnthropicApiKey  string   `json:"anthropic_api_key"`            // Anthropic API Key
	AnthropicBaseUrl string   `json:"anthropic_base_url,omitempty"` // 可选，自定义 BaseURL（代理/兼容网关）
	Model            string   `json:"model,omitempty"`              // 模型名，缺省 claude-sonnet-5
	SystemPrompt     string   `json:"system_prompt,omitempty"`      // 系统提示词
	MaxTokens        int64    `json:"max_tokens,omitempty"`         // 单次生成最大 token，缺省 4096
	WorkingDir       string   `json:"working_dir"`                  // 工具允许操作的工作目录
	BashPatterns     []string `json:"bash_patterns"`                // 允许执行的 bash 命令正则白名单
	MaxTurns         int      `json:"max_turns,omitempty"`          // agent 循环最大轮数，缺省 20
}

// loadConfig 读取并校验配置文件，填充默认值。path 可由命令行首参覆盖。
func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}
	if cfg.BotId == "" || cfg.BotSecret == "" {
		return nil, fmt.Errorf("bot_id 和 bot_secret 不能为空")
	}
	if cfg.AnthropicApiKey == "" {
		return nil, fmt.Errorf("anthropic_api_key 不能为空")
	}
	if cfg.WorkingDir == "" {
		return nil, fmt.Errorf("working_dir 不能为空")
	}
	if cfg.Model == "" {
		cfg.Model = "claude-sonnet-5"
	}
	if cfg.MaxTokens <= 0 {
		cfg.MaxTokens = 4096
	}
	if cfg.MaxTurns <= 0 {
		cfg.MaxTurns = 20
	}
	return &cfg, nil
}
