// security.go 提供 agent 工具执行的安全闸门：路径越界校验与 bash 命令白名单。
package main

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// SecurityPolicy 工具执行安全策略：限制文件访问在工作目录内、限制 bash 命令在白名单内。
type SecurityPolicy struct {
	workingDir   string           // 工具允许操作的绝对工作目录
	bashPatterns []*regexp.Regexp // 允许执行的 bash 命令正则集合
}

// NewSecurityPolicy 基于工作目录与 bash 命令正则白名单构造安全策略。
func NewSecurityPolicy(workingDir string, bashPatterns []string) (*SecurityPolicy, error) {
	absDir, err := filepath.Abs(workingDir)
	if err != nil {
		return nil, fmt.Errorf("解析工作目录失败: %w", err)
	}
	compiled := make([]*regexp.Regexp, 0, len(bashPatterns))
	for _, p := range bashPatterns {
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, fmt.Errorf("编译正则表达式 %q 失败: %w", p, err)
		}
		compiled = append(compiled, re)
	}
	return &SecurityPolicy{workingDir: absDir, bashPatterns: compiled}, nil
}

// ValidatePath 将 path 解析为工作目录内的绝对路径，越界则报错。
func (s *SecurityPolicy) ValidatePath(path string) (string, error) {
	var absPath string
	if filepath.IsAbs(path) {
		absPath = filepath.Clean(path)
	} else {
		absPath = filepath.Clean(filepath.Join(s.workingDir, path))
	}
	if !strings.HasPrefix(absPath, s.workingDir+string(filepath.Separator)) && absPath != s.workingDir {
		return "", fmt.Errorf("路径 %q 超出工作目录 %q", path, s.workingDir)
	}
	return absPath, nil
}

// ValidateBashCommand 校验命令是否命中任一允许的正则。
func (s *SecurityPolicy) ValidateBashCommand(cmd string) error {
	cmd = strings.TrimSpace(cmd)
	for _, re := range s.bashPatterns {
		if re.MatchString(cmd) {
			return nil
		}
	}
	return fmt.Errorf("命令 %q 不在允许的命令列表中", cmd)
}
