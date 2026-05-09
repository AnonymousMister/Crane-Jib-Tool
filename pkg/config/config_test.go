package config

import (
	"testing"

	"gopkg.in/yaml.v3"
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

// TestTargetsUnmarshalYAML 测试 Targets 的三种 YAML 格式解析
func TestTargetsUnmarshalYAML(t *testing.T) {
	type wrapper struct {
		To Targets `yaml:"to"`
	}

	tests := []struct {
		name      string
		yaml      string
		wantLen   int
		wantRepos []string
		wantTags  [][]string
	}{
		{
			name:      "字符串格式（向后兼容）",
			yaml:      "to: docker.io/myapp:v1",
			wantLen:   1,
			wantRepos: []string{"docker.io/myapp"},
			wantTags:  [][]string{{"v1"}},
		},
		{
			name:      "单映射格式（向后兼容）",
			yaml:      "to:\n  image: docker.io/myapp\n  tags:\n    - v1\n    - latest",
			wantLen:   1,
			wantRepos: []string{"docker.io/myapp"},
			wantTags:  [][]string{{"v1", "latest"}},
		},
		{
			name: "数组格式（多仓库）",
			yaml: "to:\n  - image: docker.io/myapp\n    tags:\n      - v1\n      - latest\n  - image: registry.cn-hangzhou.aliyuncs.com/myapp\n    tags:\n      - v1",
			wantLen:   2,
			wantRepos: []string{"docker.io/myapp", "registry.cn-hangzhou.aliyuncs.com/myapp"},
			wantTags:  [][]string{{"v1", "latest"}, {"v1"}},
		},
		{
			name: "数组中混合字符串和映射",
			yaml: "to:\n  - docker.io/myapp:v1\n  - image: registry.cn-hangzhou.aliyuncs.com/myapp\n    tags:\n      - v1\n      - latest",
			wantLen:   2,
			wantRepos: []string{"docker.io/myapp", "registry.cn-hangzhou.aliyuncs.com/myapp"},
			wantTags:  [][]string{{"v1"}, {"v1", "latest"}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var w wrapper
			if err := yaml.Unmarshal([]byte(tc.yaml), &w); err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if len(w.To) != tc.wantLen {
				t.Fatalf("Expected %d targets, got %d", tc.wantLen, len(w.To))
			}
			for i, target := range w.To {
				if target.Repository != tc.wantRepos[i] {
					t.Errorf("[%d] Expected repository %s, got %s", i, tc.wantRepos[i], target.Repository)
				}
				if len(target.Tags) != len(tc.wantTags[i]) {
					t.Errorf("[%d] Expected %d tags, got %d", i, len(tc.wantTags[i]), len(target.Tags))
					continue
				}
				for j, tag := range target.Tags {
					if tag != tc.wantTags[i][j] {
						t.Errorf("[%d][%d] Expected tag %s, got %s", i, j, tc.wantTags[i][j], tag)
					}
				}
			}
		})
	}
}
