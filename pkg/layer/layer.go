package layer

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/AnonymousMister/crane-jib-tool/pkg/config"
	"github.com/AnonymousMister/crane-jib-tool/pkg/tarutil"
)

// MergeProperties 合并全局属性和层级属性，层级属性优先级更高
func MergeProperties(global, local config.LayerProperties) config.LayerProperties {
	result := global

	// 层级属性优先级更高，覆盖全局属性
	if local.FilePermissions != "" {
		result.FilePermissions = local.FilePermissions
	}
	if local.DirectoryPermissions != "" {
		result.DirectoryPermissions = local.DirectoryPermissions
	}
	if local.User != "" {
		result.User = local.User
	}
	if local.Group != "" {
		result.Group = local.Group
	}
	if local.Timestamp != "" {
		result.Timestamp = local.Timestamp
	}

	return result
}

// ExtractPlatforms 从配置中提取平台信息
func ExtractPlatforms(fromConfig config.FromConfig) []string {
	platforms := make([]string, 0)

	if len(fromConfig.Platforms) == 0 {
		return []string{"linux/amd64"}
	}

	for _, p := range fromConfig.Platforms {
		switch v := p.(type) {
		case string:
			// 简单字符串格式，如 "linux/amd64"
			platforms = append(platforms, v)
		case map[string]interface{}:
			// 结构化格式，如 {"architecture": "arm", "tos": "linux"}
			arch, _ := v["architecture"].(string)
			tos, _ := v["os"].(string)
			if arch != "" && tos != "" {
				platforms = append(platforms, fmt.Sprintf("%s/%s", tos, arch))
			}
		}
	}

	if len(platforms) == 0 {
		return []string{"linux/amd64"}
	}

	return platforms
}

// ParseCreationTime 解析创建时间字符串，支持毫秒时间戳和 ISO 8601 格式
func ParseCreationTime(creationTimeStr string) (time.Time, error) {
	// 尝试解析为毫秒时间戳，确保整个字符串都是数字
	var ms int64
	if _, err := fmt.Sscanf(creationTimeStr, "%d", &ms); err == nil {
		// 检查是否整个字符串都是数字
		if fmt.Sprintf("%d", ms) == creationTimeStr {
			// 转换为纳秒
			return time.Unix(0, ms*1000000), nil
		}
	}

	// 尝试解析为 ISO 8601 格式，支持多种变体
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}

	for _, format := range formats {
		if ts, err := time.Parse(format, creationTimeStr); err == nil {
			return ts, nil
		}
	}

	// 尝试解析为 RFC3339 格式，忽略时区
	if ts, err := time.Parse("2006-01-02T15:04:05", creationTimeStr); err == nil {
		return ts, nil
	}

	// 解析失败，返回当前时间
	return time.Now(), fmt.Errorf("failed to parse creation time: %s", creationTimeStr)
}

// CreateTarLayer 创建 tar 包，确保 tar 文件不被包含在 tar 包中
func CreateTarLayer(contentDir, tarPath string, props config.LayerProperties) error {
	// 创建 tar 包，设置相应的属性
	if err := tarutil.CreateTar(tarPath, contentDir, tarutil.TarOptions{
		Cwd:                  contentDir,
		Files:                []string{"."},
		PreservePermissions:  false,
		FilePermissions:      props.FilePermissions,
		DirectoryPermissions: props.DirectoryPermissions,
		User:                 props.User,
		Group:                props.Group,
		Timestamp:            props.Timestamp,
	}); err != nil {
		return fmt.Errorf("creating tar layer: %w", err)
	}
	return nil
}

// matchesPattern 检查文件路径是否匹配模式（支持通配符）
func matchesPattern(filePath, pattern string) bool {
	// 如果是精确匹配，直接返回
	if filePath == pattern {
		return true
	}

	// 转换路径分隔符为正斜杠
	filePath = strings.ReplaceAll(filePath, "\\", "/")
	pattern = strings.ReplaceAll(pattern, "\\", "/")

	// 处理通配符，构建正则表达式
	var regexPattern strings.Builder

	// 遍历模式中的每个字符
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]

		if c == '*' {
			// 检查是否是 ** 通配符
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				// 匹配 **，替换为 .*（匹配任意路径）
				regexPattern.WriteString(".*")
				i++ // 跳过下一个 *
			} else {
				// 匹配 *，替换为 [^/]*（匹配单个路径段）
				regexPattern.WriteString("[^/]*")
			}
		} else if c == '?' {
			// 匹配 ?，替换为 .（匹配单个字符）
			regexPattern.WriteString(".")
		} else if c == '.' {
			// 转义 .，因为它在正则表达式中有特殊含义
			regexPattern.WriteString("\\.")
		} else {
			// 其他字符直接添加
			regexPattern.WriteByte(c)
		}
	}

	// 添加行首和行尾匹配
	finalPattern := "^" + regexPattern.String() + "$"

	// 进行正则匹配
	matched, _ := regexp.MatchString(finalPattern, filePath)
	return matched
}

// ShouldIncludeFile 检查文件是否应该被包含（根据 excludes 和 includes 规则）
func ShouldIncludeFile(filePath string, excludes, includes []string) bool {
	// 如果没有指定 excludes 和 includes，默认包含所有文件
	if len(excludes) == 0 && len(includes) == 0 {
		return true
	}

	// 检查是否在 excludes 中
	for _, exclude := range excludes {
		if matchesPattern(filePath, exclude) {
			return false
		}
	}

	// 如果指定了 includes，检查是否在 includes 中
	if len(includes) > 0 {
		for _, include := range includes {
			if matchesPattern(filePath, include) {
				return true
			}
		}
		return false
	}

	// 不在 excludes 中，且没有指定 includes，包含该文件
	return true
}

// copyDirWithFilter 递归复制目录，应用 excludes 和 includes 过滤规则
func copyDirWithFilter(srcDir, destDir string, excludes, includes []string) error {
	// 遍历源目录
	walkErr := filepath.Walk(srcDir, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 相对于源目录的路径
		relPath, err := filepath.Rel(srcDir, filePath)
		if err != nil {
			return err
		}

		// 目标文件路径
		destPath := filepath.Join(destDir, relPath)

		if info.IsDir() {
			// 创建目标目录
			return os.MkdirAll(destPath, info.Mode())
		} else {
			// 检查是否应该包含该文件
			if !ShouldIncludeFile(relPath, excludes, includes) {
				fmt.Printf("   🚫 Skipping excluded: %s\n", filePath)
				return nil
			}

			// 复制文件
			if err := copyFile(filePath, destPath); err != nil {
				return err
			}
		}

		return nil
	})

	return walkErr
}

// copyFile 复制文件
func copyFile(src, dest string) error {
	// 打开源文件
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// 创建目标文件
	destFile, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer destFile.Close()

	// 复制文件内容
	if _, err := io.Copy(destFile, srcFile); err != nil {
		return err
	}

	// 复制文件权限
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	return os.Chmod(dest, srcInfo.Mode())
}

// ProcessLayers 处理所有层，创建 tar 文件并返回层路径列表
func ProcessLayers(cfg *config.Config, rootTmpDir string) ([]string, error) {
	layerPaths := make([]string, 0, len(cfg.Layers.Entries))

	if len(cfg.Layers.Entries) == 0 {
		return layerPaths, nil
	}

	// 设置默认的用户和组
	if cfg.Layers.Properties.Group == "" {
		cfg.Layers.Properties.Group = "0"
	}
	if cfg.Layers.Properties.User == "" {
		cfg.Layers.Properties.User = "0"
	}

	// 遍历所有层
	for _, layerEntry := range cfg.Layers.Entries {
		// 合并全局属性和层级属性
		mergedProps := MergeProperties(cfg.Layers.Properties, layerEntry.Properties)

		// 创建 tar 文件路径
		layerTarPath := filepath.Join(rootTmpDir, fmt.Sprintf("%s.tar", layerEntry.Name))
		fmt.Printf("   📦 Creating layer: %s -> %s\n", layerEntry.Name, layerTarPath)

		// 创建 tar 文件
		dstFile, err := os.Create(layerTarPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create tar file %s: %w", layerTarPath, err)
		}

		// 创建 tar writer
		w := tar.NewWriter(dstFile)

		// 处理每个文件条目
		for _, file := range layerEntry.Files {
			// 获取源文件信息
			srcInfo, err := os.Stat(file.Src)
			if err != nil {
				dstFile.Close()
				os.Remove(layerTarPath)
				return nil, fmt.Errorf("failed to stat file %s: %w", file.Src, err)
			}
			mergedProps := MergeProperties(mergedProps, file.Properties)
			// 准备 tar 选项
			tarOptions := tarutil.TarOptions{
				PreservePermissions:  false,
				FilePermissions:      mergedProps.FilePermissions,
				DirectoryPermissions: mergedProps.DirectoryPermissions,
				User:                 mergedProps.User,
				Group:                mergedProps.Group,
				Timestamp:            mergedProps.Timestamp,
			}

			// 根据文件类型处理
			if srcInfo.IsDir() {
				// 源是目录，需要递归添加
				// 计算目标路径前缀（去掉末尾的/如果有的话）
				destPrefix := file.Dest
				if strings.HasSuffix(destPrefix, "/") {
					destPrefix = destPrefix[:len(destPrefix)-1]
				}

				// 遍历目录并添加到 tar
				walkErr := filepath.Walk(file.Src, func(filePath string, info os.FileInfo, err error) error {
					if err != nil {
						return err
					}

					// 相对于源目录的路径
					relPath, err := filepath.Rel(file.Src, filePath)
					if err != nil {
						return err
					}

					// 检查是否应该包含该文件
					if !ShouldIncludeFile(relPath, file.Excludes, file.Includes) {
						fmt.Printf("   🚫 Skipping excluded: %s\n", filePath)
						if info.IsDir() {
							return filepath.SkipDir
						}
						return nil
					}

					// 构建 tar 中的目标路径
					var tarPath string
					if relPath == "." {
						// 根目录，直接使用目标前缀
						tarPath = destPrefix
					} else {
						// 子文件/目录，添加到目标前缀下
						tarPath = filepath.Join(destPrefix, relPath)
					}

					// 转换为 tar 格式的路径（使用正斜杠）
					tarPath = filepath.ToSlash(tarPath)
					// 处理 Windows 驱动器号（如 C:\ -> /C/）
					if len(tarPath) > 1 && tarPath[1] == ':' {
						tarPath = "/" + strings.ToUpper(string(tarPath[0])) + tarPath[2:]
					}
					// 确保所有反斜杠都被转换为正斜杠
					tarPath = strings.ReplaceAll(tarPath, "\\", "/")

					// 添加文件到 tar
					if err := addFileToTarWithPath(w, filePath, tarPath, info, tarOptions); err != nil {
						return fmt.Errorf("failed to add file %s to tar: %w", filePath, err)
					}

					return nil
				})

				if walkErr != nil {
					w.Close()
					dstFile.Close()
					os.Remove(layerTarPath)
					return nil, fmt.Errorf("failed to walk directory %s: %w", file.Src, walkErr)
				}
			} else {
				// 源是文件，直接添加
				// 检查是否应该包含该文件
				if !ShouldIncludeFile(filepath.Base(file.Src), file.Excludes, file.Includes) {
					fmt.Printf("   🚫 Skipping excluded: %s\n", file.Src)
					continue
				}

				// 构建 tar 中的目标路径
				var tarPath string
				if strings.HasSuffix(file.Dest, "/") {
					// 目标是目录，使用源文件名
					tarPath = filepath.Join(file.Dest, filepath.Base(file.Src))
				} else {
					// 目标是文件，直接使用
					tarPath = file.Dest
				}

				// 转换为 tar 格式的路径（使用正斜杠）
				tarPath = filepath.ToSlash(tarPath)
				// 处理 Windows 驱动器号（如 C:\ -> /C/）
				if len(tarPath) > 1 && tarPath[1] == ':' {
					tarPath = "/" + strings.ToUpper(string(tarPath[0])) + tarPath[2:]
				}
				// 确保所有反斜杠都被转换为正斜杠
				tarPath = strings.ReplaceAll(tarPath, "\\", "/")

				// 添加文件到 tar
				if err := addFileToTarWithPath(w, file.Src, tarPath, srcInfo, tarOptions); err != nil {
					w.Close()
					dstFile.Close()
					os.Remove(layerTarPath)
					return nil, fmt.Errorf("failed to add file %s to tar: %w", file.Src, err)
				}
			}
		}

		// 关闭 tar writer
		if err := w.Close(); err != nil {
			dstFile.Close()
			os.Remove(layerTarPath)
			return nil, fmt.Errorf("failed to close tar writer: %w", err)
		}

		// 关闭目标文件
		if err := dstFile.Close(); err != nil {
			os.Remove(layerTarPath)
			return nil, fmt.Errorf("failed to close tar file: %w", err)
		}

		// 添加到层路径列表
		layerPaths = append(layerPaths, layerTarPath)
	}

	return layerPaths, nil
}

// addFileToTarWithPath 将文件添加到 tar 包，支持自定义 tar 内路径
func addFileToTarWithPath(w *tar.Writer, filePath, tarPath string, info os.FileInfo, opt tarutil.TarOptions) error {
	// 打开文件（如果是目录则不需要）
	var file *os.File
	var err error
	if !info.IsDir() {
		file, err = os.Open(filePath)
		if err != nil {
			return fmt.Errorf("failed to open file %s: %w", filePath, err)
		}
		defer file.Close()
	}

	// 创建 tar 头
	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return fmt.Errorf("failed to create tar header for %s: %w", filePath, err)
	}

	// 设置自定义 tar 路径
	header.Name = tarPath

	// 设置文件权限
	if !opt.PreservePermissions {
		// 使用自定义或默认权限
		if info.IsDir() {
			// 目录权限
			if opt.DirectoryPermissions != "" {
				// 解析自定义目录权限
				var dirMode int64
				if _, err := fmt.Sscanf(opt.DirectoryPermissions, "%o", &dirMode); err == nil {
					header.Mode = dirMode
				} else {
					// 解析失败，使用默认权限
					header.Mode = int64(0755)
				}
			} else {
				// 使用默认目录权限
				header.Mode = int64(0755)
			}
		} else {
			// 文件权限
			if opt.FilePermissions != "" {
				// 解析自定义文件权限
				var fileMode int64
				if _, err := fmt.Sscanf(opt.FilePermissions, "%o", &fileMode); err == nil {
					header.Mode = fileMode
				} else {
					// 解析失败，使用默认权限
					header.Mode = int64(0644)
				}
			} else {
				// 使用默认文件权限
				header.Mode = int64(0644)
			}
		}
	}

	// 设置用户和组
	if opt.User != "" {
		var uid int
		if _, err := fmt.Sscanf(opt.User, "%d", &uid); err == nil {
			header.Uid = uid
		}
	}
	if opt.Group != "" {
		var gid int
		if _, err := fmt.Sscanf(opt.Group, "%d", &gid); err == nil {
			header.Gid = gid
		}
	}

	// 设置修改时间
	if opt.Timestamp != "" {
		// 尝试解析为时间戳（毫秒）
		var ms int64
		if _, err := fmt.Sscanf(opt.Timestamp, "%d", &ms); err == nil {
			// 转换为纳秒
			timestamp := time.Unix(0, ms*1000000)
			header.ModTime = timestamp
			header.AccessTime = timestamp
			header.ChangeTime = timestamp
		} else {
			// 尝试解析为 ISO 8601 格式
			if ts, err := time.Parse(time.RFC3339, opt.Timestamp); err == nil {
				header.ModTime = ts
				header.AccessTime = ts
				header.ChangeTime = ts
			}
			// 解析失败则使用文件的修改时间（已在 FileInfoHeader 中设置）
		}
	}

	// 写入 tar 头
	if err := w.WriteHeader(header); err != nil {
		return fmt.Errorf("failed to write tar header for %s: %w", filePath, err)
	}

	// 如果是目录，不需要写入内容
	if info.IsDir() {
		return nil
	}

	// 写入文件内容
	if _, err := io.Copy(w, file); err != nil {
		return fmt.Errorf("failed to write file content for %s: %w", filePath, err)
	}

	return nil
}
