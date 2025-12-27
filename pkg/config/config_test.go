package config

import (
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
