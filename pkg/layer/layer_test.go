package layer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AnonymousMister/crane-jib-tool/pkg/config"
)

// TestExtractPlatforms 测试提取平台信息功能
func TestExtractPlatforms(t *testing.T) {
	// 测试用例：空平台配置
	fromConfig := config.FromConfig{
		Image:     "ubuntu",
		Platforms: nil,
	}
	platforms := ExtractPlatforms(fromConfig)
	if len(platforms) != 1 || platforms[0] != "linux/amd64" {
		t.Errorf("Expected [linux/amd64], got %v", platforms)
	}

	// 测试用例：字符串格式的平台
	fromConfig = config.FromConfig{
		Image: "ubuntu",
		Platforms: []interface{}{
			"linux/amd64",
			"linux/arm64",
		},
	}
	platforms = ExtractPlatforms(fromConfig)
	expected := []string{"linux/amd64", "linux/arm64"}
	if len(platforms) != len(expected) {
		t.Errorf("Expected %v, got %v", expected, platforms)
	}
	for i, p := range platforms {
		if p != expected[i] {
			t.Errorf("Expected %v, got %v", expected, platforms)
			break
		}
	}

	// 测试用例：结构化格式的平台
	fromConfig = config.FromConfig{
		Image: "ubuntu",
		Platforms: []interface{}{
			map[string]interface{}{
				"architecture": "amd64",
				"os":           "linux",
			},
			map[string]interface{}{
				"architecture": "arm",
				"os":           "linux",
			},
		},
	}
	platforms = ExtractPlatforms(fromConfig)
	expected = []string{"linux/amd64", "linux/arm"}
	if len(platforms) != len(expected) {
		t.Errorf("Expected %v, got %v", expected, platforms)
	}
	for i, p := range platforms {
		if p != expected[i] {
			t.Errorf("Expected %v, got %v", expected, platforms)
			break
		}
	}
}

// TestParseCreationTime 测试解析创建时间功能
func TestParseCreationTime(t *testing.T) {
	// 测试用例：毫秒时间戳
	ts, err := ParseCreationTime("1000")
	if err != nil {
		t.Errorf("Expected no error parsing 1000, got %v", err)
	}
	expected := ts.UnixNano()
	if expected != 1000*1000000 {
		t.Errorf("Expected %d, got %d", 1000*1000000, expected)
	}

	// 测试用例：ISO 8601 格式
	ts, err = ParseCreationTime("2020-01-01T00:00:00Z")
	if err != nil {
		t.Errorf("Expected no error parsing ISO 8601 date, got %v", err)
	}
	// 验证年份和月份
	if ts.Year() != 2020 || ts.Month() != 1 || ts.Day() != 1 {
		t.Errorf("Expected 2020-01-01, got %v", ts)
	}

	// 测试用例：无效格式
	ts, err = ParseCreationTime("invalid-date")
	if err == nil {
		t.Error("Expected error parsing invalid date")
	}
	// 应该返回当前时间
	if ts.IsZero() {
		t.Error("Expected non-zero time for invalid date")
	}
}

// TestMergeProperties 测试合并属性功能
func TestMergeProperties(t *testing.T) {
	globalProps := config.LayerProperties{
		FilePermissions:      "644",
		DirectoryPermissions: "755",
		User:                 "0",
		Group:                "0",
		Timestamp:            "1000",
	}

	localProps := config.LayerProperties{
		FilePermissions: "755",
		User:            "1000",
	}

	merged := MergeProperties(globalProps, localProps)

	// 验证合并结果
	if merged.FilePermissions != "755" {
		t.Errorf("Expected FilePermissions=755, got %s", merged.FilePermissions)
	}
	if merged.DirectoryPermissions != "755" {
		t.Errorf("Expected DirectoryPermissions=755, got %s", merged.DirectoryPermissions)
	}
	if merged.User != "1000" {
		t.Errorf("Expected User=1000, got %s", merged.User)
	}
	if merged.Group != "0" {
		t.Errorf("Expected Group=0, got %s", merged.Group)
	}
	if merged.Timestamp != "1000" {
		t.Errorf("Expected Timestamp=1000, got %s", merged.Timestamp)
	}
}

// TestMatchesPattern 测试模式匹配功能
func TestMatchesPattern(t *testing.T) {
	// 测试用例：精确匹配
	if !matchesPattern("test.txt", "test.txt") {
		t.Error("Expected test.txt to match test.txt")
	}

	// 测试用例：* 通配符
	if !matchesPattern("test.txt", "*.txt") {
		t.Error("Expected test.txt to match *.txt")
	}

	// 测试用例：** 通配符
	if !matchesPattern("dir/subdir/test.txt", "**/*.txt") {
		t.Error("Expected dir/subdir/test.txt to match **/*.txt")
	}

	// 测试用例：? 通配符
	if !matchesPattern("test.txt", "test.?xt") {
		t.Error("Expected test.txt to match test.?xt")
	}

	// 测试用例：不匹配
	if matchesPattern("test.txt", "*.md") {
		t.Error("Expected test.txt to not match *.md")
	}
}

// TestShouldIncludeFile 测试文件过滤功能
func TestShouldIncludeFile(t *testing.T) {
	// 测试用例：空过滤规则，应该包含所有文件
	if !ShouldIncludeFile("test.txt", nil, nil) {
		t.Error("Expected test.txt to be included with no filters")
	}

	// 测试用例：只包含特定文件
	if !ShouldIncludeFile("test.txt", nil, []string{"*.txt"}) {
		t.Error("Expected test.txt to be included with include=*.txt")
	}
	if ShouldIncludeFile("test.md", nil, []string{"*.txt"}) {
		t.Error("Expected test.md to be excluded with include=*.txt")
	}

	// 测试用例：排除特定文件
	if !ShouldIncludeFile("test.txt", []string{"*.md"}, nil) {
		t.Error("Expected test.txt to be included with exclude=*.md")
	}
	if ShouldIncludeFile("test.md", []string{"*.md"}, nil) {
		t.Error("Expected test.md to be excluded with exclude=*.md")
	}

	// 测试用例：同时包含和排除
	if ShouldIncludeFile("test.txt", []string{"*.txt"}, []string{"test.*"}) {
		t.Error("Expected test.txt to be excluded with exclude=*.txt and include=test.*")
	}
	if !ShouldIncludeFile("test.go", []string{"*.md"}, []string{"*.go"}) {
		t.Error("Expected test.go to be included with exclude=*.md and include=*.go")
	}
}

// TestCreateTarLayer 测试创建 tar 包功能
func TestCreateTarLayer(t *testing.T) {
	// 创建临时目录用于测试
	tmpDir, err := os.MkdirTemp("", "test-create-tar-layer")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建测试子目录用于存放要打包的文件
	contentDir := filepath.Join(tmpDir, "content")
	if err := os.MkdirAll(contentDir, 0755); err != nil {
		t.Fatalf("Failed to create content dir: %v", err)
	}

	// 创建测试文件
	testFile := filepath.Join(contentDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// 创建 tar 包，确保 tar 文件不被包含在 tar 包中
	tarPath := filepath.Join(tmpDir, "test.tar")
	props := config.LayerProperties{
		FilePermissions:      "644",
		DirectoryPermissions: "755",
	}
	if err := CreateTarLayer(contentDir, tarPath, props); err != nil {
		t.Fatalf("Failed to create tar layer: %v", err)
	}

	// 验证 tar 包存在
	if _, err := os.Stat(tarPath); os.IsNotExist(err) {
		t.Error("Expected tar file to be created")
	}
}

// TestCopyFile 测试复制文件功能
func TestCopyFile(t *testing.T) {
	// 创建临时目录用于测试
	tmpDir, err := os.MkdirTemp("", "test-copy-file")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建测试文件
	srcFile := filepath.Join(tmpDir, "src.txt")
	dstFile := filepath.Join(tmpDir, "dst.txt")
	content := []byte("test content")
	if err := os.WriteFile(srcFile, content, 0644); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// 复制文件
	if err := copyFile(srcFile, dstFile); err != nil {
		t.Fatalf("Failed to copy file: %v", err)
	}

	// 验证文件已复制
	if _, err := os.Stat(dstFile); os.IsNotExist(err) {
		t.Error("Expected destination file to be created")
	}

	// 验证文件内容一致
	dstContent, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("Failed to read destination file: %v", err)
	}
	if !strings.EqualFold(string(content), string(dstContent)) {
		t.Errorf("Expected content %q, got %q", content, dstContent)
	}
}

// TestCopyDirWithFilter 测试带过滤的目录复制功能
func TestCopyDirWithFilter(t *testing.T) {
	// 创建临时目录用于测试
	srcDir, err := os.MkdirTemp("", "test-copy-dir-src")
	if err != nil {
		t.Fatalf("Failed to create source temp dir: %v", err)
	}
	defer os.RemoveAll(srcDir)

	dstDir, err := os.MkdirTemp("", "test-copy-dir-dst")
	if err != nil {
		t.Fatalf("Failed to create destination temp dir: %v", err)
	}
	defer os.RemoveAll(dstDir)

	// 创建测试文件
	createTestFile := func(dir, name, content string) {
		filePath := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			t.Fatalf("Failed to create dir for %s: %v", filePath, err)
		}
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", filePath, err)
		}
	}

	createTestFile(srcDir, "file1.txt", "content1")
	createTestFile(srcDir, "file2.md", "content2")
	createTestFile(srcDir, "subdir/file3.txt", "content3")
	createTestFile(srcDir, "subdir/file4.md", "content4")

	// 测试用例：只复制 txt 文件
	excludes := []string{"*.md"}
	includes := []string{"*.txt", "**/*.txt"}
	if err := copyDirWithFilter(srcDir, dstDir, excludes, includes); err != nil {
		t.Fatalf("Failed to copy dir with filter: %v", err)
	}

	// 验证复制结果
	expectedFiles := []string{
		"file1.txt",
		"subdir/file3.txt",
	}

	for _, expectedFile := range expectedFiles {
		expectedPath := filepath.Join(dstDir, expectedFile)
		if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
			t.Errorf("Expected file %s to be copied", expectedFile)
		}
	}

	// 验证排除的文件没有被复制
	excludedFiles := []string{
		"file2.md",
		"subdir/file4.md",
	}

	for _, excludedFile := range excludedFiles {
		excludedPath := filepath.Join(dstDir, excludedFile)
		if _, err := os.Stat(excludedPath); err == nil {
			t.Errorf("Expected file %s to be excluded", excludedFile)
		}
	}
}
