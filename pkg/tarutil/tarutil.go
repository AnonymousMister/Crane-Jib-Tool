// Copyright 2021 Google LLC All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package tarutil provides a cross-platform utility for creating tar archives.
package tarutil

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// TarOptions 定义了创建 tar 包的选项
type TarOptions struct {
	// Compress 是否启用压缩（目前仅支持不压缩）
	Compress bool
	// Gzip 是否使用 gzip 压缩（目前仅支持不压缩）
	Gzip bool
	// Cwd 设置 tar 包的当前工作目录
	Cwd string
	// Files 指定要包含的文件或目录
	Files []string
	// PreservePermissions 是否保留文件权限
	PreservePermissions bool
	// FilePermissions 指定文件的权限（八进制字符串，如 "644"）
	FilePermissions string
	// DirectoryPermissions 指定目录的权限（八进制字符串，如 "755"）
	DirectoryPermissions string
	// User 指定文件的用户 ID
	User string
	// Group 指定文件的组 ID
	Group string
	// Timestamp 指定文件的时间戳
	Timestamp string
}

// CreateTar 创建 tar 包，支持跨平台
// dst: 目标 tar 文件路径
// srcDir: 源目录
// cwd: 相对路径的基准目录
func CreateTar(dst, srcDir string, options ...TarOptions) error {
	// 解析选项
	opt := TarOptions{}
	if len(options) > 0 {
		opt = options[0]
	}

	// 确保目标目录存在
	dstDir := path.Dir(dst)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory %s: %w", dstDir, err)
	}

	// 创建目标文件
	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create tar file %s: %w", dst, err)
	}
	defer dstFile.Close()

	// 创建 tar writer
	w := tar.NewWriter(dstFile)
	defer w.Close()

	// 如果没有指定文件，默认添加当前目录下的所有内容
	files := opt.Files
	if len(files) == 0 {
		files = []string{"."}
	}

	// 设置当前工作目录
	cwd := srcDir
	if opt.Cwd != "" {
		cwd = opt.Cwd
	}

	// 添加文件到 tar 包
	for _, file := range files {
		if err := addFileToTar(w, cwd, file, opt); err != nil {
			return fmt.Errorf("failed to add file %s to tar: %w", file, err)
		}
	}

	// 关闭 tar writer
	if err := w.Close(); err != nil {
		return fmt.Errorf("failed to close tar writer: %w", err)
	}

	// 关闭目标文件
	if err := dstFile.Close(); err != nil {
		return fmt.Errorf("failed to close tar file: %w", err)
	}

	return nil
}

// addFileToTar 添加文件或目录到 tar 包
func addFileToTar(w *tar.Writer, cwd, file string, opt TarOptions) error {
	// 构建完整路径
	fullPath := file
	if !filepath.IsAbs(file) {
		fullPath = filepath.Join(cwd, file)
	}

	// 获取文件信息
	info, err := os.Stat(fullPath)
	if err != nil {
		return fmt.Errorf("failed to stat file %s: %w", fullPath, err)
	}

	// 如果是目录，递归添加
	if info.IsDir() {
		return addDirectoryToTar(w, cwd, fullPath, opt)
	}

	// 添加单个文件
	return addSingleFileToTar(w, cwd, fullPath, info, opt)
}

// addDirectoryToTar 递归添加目录到 tar 包
func addDirectoryToTar(w *tar.Writer, cwd, dirPath string, opt TarOptions) error {
	// 遍历目录内容
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %w", dirPath, err)
	}

	for _, entry := range entries {
		// 构建完整路径
		entryPath := filepath.Join(dirPath, entry.Name())
		// 获取文件信息
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("failed to get info for %s: %w", entryPath, err)
		}

		// 递归添加
		if info.IsDir() {
			if err := addDirectoryToTar(w, cwd, entryPath, opt); err != nil {
				return err
			}
		} else {
			if err := addSingleFileToTar(w, cwd, entryPath, info, opt); err != nil {
				return err
			}
		}
	}

	return nil
}

// addSingleFileToTar 添加单个文件到 tar 包
func addSingleFileToTar(w *tar.Writer, cwd, filePath string, info os.FileInfo, opt TarOptions) error {
	// 打开文件
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close()

	// 构建相对路径（确保使用正斜杠）
	relPath, err := filepath.Rel(cwd, filePath)
	if err != nil {
		return fmt.Errorf("failed to get relative path for %s: %w", filePath, err)
	}
	// 转换为 tar 格式的路径（使用正斜杠）
	tarPath := filepath.ToSlash(relPath)

	// 创建 tar 头
	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return fmt.Errorf("failed to create tar header for %s: %w", filePath, err)
	}

	// 设置正确的文件名
	header.Name = tarPath

	// 修复 Windows 平台的路径分隔符
	if runtime.GOOS == "windows" {
		header.Name = strings.ReplaceAll(header.Name, "\\", "/")
	}

	// 设置文件权限
	if opt.PreservePermissions {
		// 保留原始权限
		header.Mode = int64(info.Mode())
	} else {
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
			} else {
				// 解析失败，使用文件的修改时间
				header.ModTime = info.ModTime()
				header.AccessTime = info.ModTime()
				header.ChangeTime = info.ModTime()
			}
		}
	} else {
		// 使用文件的修改时间
		header.ModTime = info.ModTime()
		header.AccessTime = info.ModTime()
		header.ChangeTime = info.ModTime()
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

// ExtractTar 解压缩 tar 包（可选功能，方便测试和使用）
func ExtractTar(src, dst string) error {
	// 打开 tar 文件
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open tar file %s: %w", src, err)
	}
	defer srcFile.Close()

	// 创建 tar reader
	r := tar.NewReader(srcFile)

	// 确保目标目录存在
	if err := os.MkdirAll(dst, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory %s: %w", dst, err)
	}

	// 遍历 tar 文件中的条目
	for {
		header, err := r.Next()
		if err == io.EOF {
			break // 结束
		}
		if err != nil {
			return fmt.Errorf("failed to read tar entry: %w", err)
		}

		// 构建目标路径
		destPath := filepath.Join(dst, header.Name)

		// 根据条目类型处理
		switch header.Typeflag {
		case tar.TypeDir:
			// 创建目录
			if err := os.MkdirAll(destPath, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", destPath, err)
			}
		case tar.TypeReg:
			// 创建文件
			if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
				return fmt.Errorf("failed to create parent directory for %s: %w", destPath, err)
			}
			destFile, err := os.Create(destPath)
			if err != nil {
				return fmt.Errorf("failed to create file %s: %w", destPath, err)
			}

			// 写入文件内容
			if _, err := io.Copy(destFile, r); err != nil {
				destFile.Close()
				return fmt.Errorf("failed to write file %s: %w", destPath, err)
			}
			destFile.Close()

			// 设置文件权限
			if err := os.Chmod(destPath, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("failed to set permissions for %s: %w", destPath, err)
			}
		case tar.TypeSymlink:
			// 创建符号链接（仅在非 Windows 平台支持）
			if runtime.GOOS != "windows" {
				if err := os.Symlink(header.Linkname, destPath); err != nil {
					return fmt.Errorf("failed to create symlink %s: %w", destPath, err)
				}
			}
		}
	}

	return nil
}
