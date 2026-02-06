package config

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 定义了镜像构建的配置结构
type Config struct {
	APIVersion   string            `yaml:"apiVersion"`
	Kind         string            `yaml:"kind"`
	From         FromConfig        `yaml:"from"`
	CreationTime string            `yaml:"creationTime"`
	Format       string            `yaml:"format"`
	Environment  map[string]string `yaml:"environment"`
	Labels       map[string]string `yaml:"labels"`
	Volumes      []string          `yaml:"volumes"`
	ExposedPorts []string          `yaml:"exposedPorts"`
	User         string            `yaml:"user"`
	WorkingDir   string            `yaml:"workingDirectory"`
	Entrypoint   []string          `yaml:"entrypoint"`
	Cmd          []string          `yaml:"cmd"`
	Layers       LayerConfig       `yaml:"layers"`
	To           Tag               `yaml:"to"`
	Insecure     bool              `yaml:"insecure"`
}

type Tag struct {
	Repository string   `yaml:"image"`
	Tags       []string `yaml:"tags"`
}

func (t *Tag) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		lastIndex := strings.LastIndex(value.Value, ":")
		if lastIndex == -1 {
			return fmt.Errorf("invalid tag format: %s", value.Value)
		}
		repository := value.Value[:lastIndex]
		tag := value.Value[lastIndex+1:]
		t.Repository = repository
		t.Tags = []string{tag}
		return nil
	} else if value.Kind == yaml.MappingNode {
		type rawTag Tag
		return value.Decode((*rawTag)(t))
	}
	return fmt.Errorf("unsupported tag type: %s", value.Tag)
}

// FromConfig 定义了基础镜像的配置
type FromConfig struct {
	Image     string        `yaml:"image"`
	Platforms []interface{} `yaml:"platforms"`
}

// LayerConfig 定义了所有层的配置
type LayerConfig struct {
	Properties LayerProperties `yaml:"properties"`
	Entries    []LayerEntry    `yaml:"entries"`
}

// LayerProperties 定义了层的公共属性
type LayerProperties struct {
	FilePermissions      string `yaml:"filePermissions"`
	DirectoryPermissions string `yaml:"directoryPermissions"`
	User                 string `yaml:"user"`
	Group                string `yaml:"group"`
	Timestamp            string `yaml:"timestamp"`
}

// LayerEntry 定义了单个层的配置
type LayerEntry struct {
	Name       string          `yaml:"name"`
	Properties LayerProperties `yaml:"properties"`
	Files      []FileConfig    `yaml:"files"`
}

// FileConfig 定义了文件复制的配置
type FileConfig struct {
	Src        string          `yaml:"src"`
	Dest       string          `yaml:"dest"`
	Excludes   []string        `yaml:"excludes"`
	Includes   []string        `yaml:"includes"`
	Properties LayerProperties `yaml:"properties"`
}

// BuildVarPool 构建变量池，从环境变量、变量文件和命令行参数中收集变量
func BuildVarPool(valFiles []string, vals map[string]string) map[string]string {
	// 1. 初始化变量池，包含环境变量和默认的时间戳
	now := time.Now()
	defaultTimestamp := fmt.Sprintf("%d%02d%02d%02d%02d%02d",
		now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), now.Second())

	pool := make(map[string]string)
	// 添加环境变量
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			pool[parts[0]] = parts[1]
		}
	}
	// 添加默认时间戳
	pool["TimestampTag"] = defaultTimestamp

	// 2. 添加变量文件中的变量
	for _, file := range valFiles {
		content, err := os.ReadFile(file)
		if err != nil {
			fmt.Printf("Warning: failed to read variable file %s: %v\n", file, err)
			continue
		}

		var fileVars map[string]string
		if err := yaml.Unmarshal(content, &fileVars); err != nil {
			fmt.Printf("Warning: failed to parse variable file %s: %v\n", file, err)
			continue
		}

		for k, v := range fileVars {
			pool[k] = v
		}
	}

	// 3. 添加命令行参数中的变量（优先级最高）
	for k, v := range vals {
		pool[k] = v
	}

	return pool
}

// ReplaceVars 替换字符串中的变量，格式为 ${var}
func ReplaceVars(str string, pool map[string]string) (string, error) {
	re := regexp.MustCompile(`\$\{(.*?)\}`)
	var err error
	result := re.ReplaceAllStringFunc(str, func(match string) string {
		key := strings.Trim(match, "${}")
		if val, ok := pool[key]; ok {
			return val
		}
		err = fmt.Errorf("undefined variable: ${%s}", key)
		return match
	})
	return result, err
}

// ParseConfig 解析配置文件并应用变量替换
func ParseConfig(configPath string, varPool map[string]string) (*Config, error) {
	// 1. 检查配置文件是否存在
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file %s does not exist", configPath)
	}

	// 2. 读取配置文件内容
	content, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// 3. 应用变量替换到配置文件内容
	contentStr, err := ReplaceVars(string(content), varPool)
	if err != nil {
		return nil, err
	}

	// 4. 解析 YAML 配置
	var cfg Config
	if err := yaml.Unmarshal([]byte(contentStr), &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// 5. 验证必要字段
	if cfg.From.Image == "" {
		return nil, errors.New("from.image field is required in config file")
	}
	if cfg.To.Repository == "" {
		return nil, errors.New("to field is required in config file")
	}

	return &cfg, nil
}
