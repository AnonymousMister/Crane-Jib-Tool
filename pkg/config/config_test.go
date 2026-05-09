package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestBuildVarPool 测试构建变量池功能
func TestBuildVarPool(t *testing.T) {
	// 测试用例：空输入
	vars := BuildVarPool(nil, nil)
	if vars == nil {
		t.Error("Expected non-nil vars map")
	}

	// 测试用例：添加命令行参数
	cmdVars := map[string]string{"TestVar": "test-value"}
	vars = BuildVarPool(nil, cmdVars)
	if vars["TestVar"] != "test-value" {
		t.Errorf("Expected TestVar=test-value, got %s=%s", "TestVar", vars["TestVar"])
	}

	// 测试用例：验证默认时间戳存在
	if _, ok := vars["TimestampTag"]; !ok {
		t.Error("Expected TimestampTag in vars map")
	}
}

// TestReplaceVars 测试变量替换功能
func TestReplaceVars(t *testing.T) {
	varPool := map[string]string{
		"TestVar":    "test-value",
		"AnotherVar": "another-value",
	}

	// 测试用例：基本替换
	testStr := "This is a ${TestVar} and ${AnotherVar}"
	result, err := ReplaceVars(testStr, varPool)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	expected := "This is a test-value and another-value"
	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}

	// 测试用例：不存在的变量应该报错
	testStr = "This is a ${NonExistentVar}"
	result, err = ReplaceVars(testStr, varPool)
	if err == nil {
		t.Error("Expected error for undefined variable, got nil")
	}

	// 测试用例：空字符串
	testStr = ""
	result, err = ReplaceVars(testStr, varPool)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	expected = ""
	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}

func TestExecuteTemplate_BasicConditional(t *testing.T) {
	content := `layers:
  entries:
    - name: app
{{- if .EnableMonitoring}}
    - name: monitoring
{{- end}}`

	vars := map[string]string{"EnableMonitoring": "true"}
	result, err := ExecuteTemplate(content, vars)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !strings.Contains(result, "monitoring") {
		t.Error("Expected monitoring section when EnableMonitoring is set")
	}

	vars = map[string]string{}
	result, err = ExecuteTemplate(content, vars)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if strings.Contains(result, "monitoring") {
		t.Error("Expected no monitoring section when EnableMonitoring is not set")
	}
}

func TestExecuteTemplate_NoTemplateDirectives(t *testing.T) {
	content := `from:
  image: ubuntu
to: registry.example.com/app:${Version}`

	vars := map[string]string{"Version": "1.0"}
	result, err := ExecuteTemplate(content, vars)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !strings.Contains(result, "${Version}") {
		t.Error("ExecuteTemplate should not touch ${Variable} syntax")
	}
	if result != content {
		t.Errorf("Content without template directives should pass through unchanged, got: %s", result)
	}
}

func TestExecuteTemplate_DefaultFunc(t *testing.T) {
	content := `image: {{default "ubuntu:latest" .BaseImage}}`

	vars := map[string]string{}
	result, err := ExecuteTemplate(content, vars)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !strings.Contains(result, "ubuntu:latest") {
		t.Errorf("Expected default value, got: %s", result)
	}

	vars = map[string]string{"BaseImage": "alpine:3.18"}
	result, err = ExecuteTemplate(content, vars)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !strings.Contains(result, "alpine:3.18") {
		t.Errorf("Expected provided value, got: %s", result)
	}
}

func TestExecuteTemplate_InvalidSyntax(t *testing.T) {
	content := `{{if .Foo}unclosed`
	vars := map[string]string{}
	_, err := ExecuteTemplate(content, vars)
	if err == nil {
		t.Error("Expected error for invalid template syntax")
	}
	if !strings.Contains(err.Error(), "template parse error") {
		t.Errorf("Expected 'template parse error' in message, got: %v", err)
	}
}

func TestExecuteTemplate_CustomFuncs(t *testing.T) {
	content := `name: {{upper .AppName}}
check: {{contains .Tags "prod"}}`

	vars := map[string]string{"AppName": "myapp", "Tags": "dev,prod,staging"}
	result, err := ExecuteTemplate(content, vars)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !strings.Contains(result, "MYAPP") {
		t.Errorf("Expected upper case, got: %s", result)
	}
	if !strings.Contains(result, "true") {
		t.Errorf("Expected contains=true, got: %s", result)
	}
}

func TestParseConfig_TemplateAndVarReplacement(t *testing.T) {
	dir := t.TempDir()
	configContent := `apiVersion: jib/v1alpha1
kind: BuildFile
from:
  image: {{default "ubuntu:latest" .BaseImage}}
{{- if .EnableArm}}
  platforms:
    - "linux/amd64"
    - "linux/arm64"
{{- end}}
to: ${IMAGE}:${Version}
layers:
  entries:
    - name: app
      files:
        - src: ./app.jar
          dest: /app/app.jar`

	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	varPool := map[string]string{
		"IMAGE":     "registry.example.com/app",
		"Version":   "1.0",
		"EnableArm": "true",
	}

	cfg, err := ParseConfig(configPath, varPool)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if cfg.From.Image != "ubuntu:latest" {
		t.Errorf("Expected default base image, got: %s", cfg.From.Image)
	}
	if cfg.To.Repository != "registry.example.com/app" {
		t.Errorf("Expected repository from ${IMAGE}, got: %s", cfg.To.Repository)
	}
	if len(cfg.From.Platforms) != 2 {
		t.Errorf("Expected 2 platforms with EnableArm, got: %d", len(cfg.From.Platforms))
	}
}
