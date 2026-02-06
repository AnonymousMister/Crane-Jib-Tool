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

package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AnonymousMister/crane-jib-tool/pkg/config"
	"github.com/AnonymousMister/crane-jib-tool/pkg/layer"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/spf13/cobra"
)

// ValFlag 用于处理命令行中的键值对参数
type ValFlag map[string]string

// Set 实现了 pflag.Value 接口
func (v *ValFlag) Set(value string) error {
	parts := strings.SplitN(value, "=", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid value format: %s, expected key=value", value)
	}
	(*v)[parts[0]] = parts[1]
	return nil
}

// String 实现了 pflag.Value 接口
func (v *ValFlag) String() string {
	return fmt.Sprintf("%v", *v)
}

// Type 实现了 pflag.Value 接口
func (v *ValFlag) Type() string {
	return "key=value"
}

// NewCmdCreate creates a new cobra.Command for the create subcommand.
func NewCmdCreate(options *[]crane.Option) *cobra.Command {
	// 配置文件相关参数
	var configFile string
	var valFiles []string
	var vals ValFlag = make(ValFlag)

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new image from an existing one using a configuration file.",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, args []string) error {
			// 1. 检查配置文件是否提供
			if configFile == "" {
				return errors.New("--config flag is required")
			}

			// 2. 构建变量池
			fmt.Println("🔧 Building variable pool...")
			varPool := config.BuildVarPool(valFiles, vals)

			// 3. 解析配置文件
			fmt.Println("📄 Parsing configuration file...")
			cfg, err := config.ParseConfig(configFile, varPool)
			if err != nil {
				return fmt.Errorf("failed to parse config file: %w", err)
			}

			// 4. 从配置中提取平台信息
			platforms := layer.ExtractPlatforms(cfg.From)
			fmt.Printf("🎯 Platforms: %v\n", platforms)

			// 5. 创建临时目录用于存储中间产物
			rootTmpDir, err := os.MkdirTemp("", "crane-job-")
			if err != nil {
				return fmt.Errorf("failed to create root temp dir: %w", err)
			}
			defer os.RemoveAll(rootTmpDir)
			fmt.Printf("📁 Using temp directory: %s\n", rootTmpDir)

			// OCI layout 目录
			ociLayoutDir := filepath.Join(rootTmpDir, "oci-layout")
			if err := os.MkdirAll(ociLayoutDir, 0755); err != nil {
				return fmt.Errorf("failed to create OCI layout dir: %w", err)
			}

			// 6. 处理所有层，创建 tar 文件
			fmt.Printf("📦 Preparing %d layers...\n", len(cfg.Layers.Entries))
			layerPaths, err := layer.ProcessLayers(cfg, rootTmpDir)
			if err != nil {
				return fmt.Errorf("failed to process layers: %w", err)
			}

			var createdTime = time.Now()

			// 当前创建时间
			if cfg.CreationTime != "" {
				createdTime, err := layer.ParseCreationTime(cfg.CreationTime)
				if err != nil {
					fmt.Printf("   ⚠️  Warning: %v, using current time instead\n", err)
					createdTime = time.Now()
				}
				fmt.Printf("   ⚙️  Setting creation time: %v\n", createdTime)
			}

			// 7. 处理每个平台
			// 存储每个平台的镜像信息
			platformImageRefs := make([]string, 0, len(platforms))

			for _, targetPlatform := range platforms {
				fmt.Printf("\n🔨 Processing platform: %s\n", targetPlatform)

				// 解析平台字符串
				platform, err := parsePlatform(targetPlatform)
				if err != nil {
					return fmt.Errorf("parsing platform %s: %w", targetPlatform, err)
				}

				// 8. 拉取基础镜像
				fmt.Printf("   📥 Pulling base image: %s\n", cfg.From.Image)
				img, err := crane.Pull(cfg.From.Image, append(*options, crane.WithPlatform(platform))...)
				if err != nil {
					return fmt.Errorf("pulling base image %s: %w", cfg.From.Image, err)
				}

				// 9. 添加层
				if len(layerPaths) > 0 {
					fmt.Printf("   🧩 Adding %d layers...\n", len(layerPaths))
					img, err = crane.Append(img, layerPaths...)
					if err != nil {
						return fmt.Errorf("adding layer %w", err)
					}
				}

				// 10. 获取并修改配置
				imgCfg, err := img.ConfigFile()
				if err != nil {
					return fmt.Errorf("getting config file: %w", err)
				}
				imgCfg = imgCfg.DeepCopy()

				// 10.1 设置创建时间
				imgCfg.Created = v1.Time{Time: createdTime}

				// 11. 设置环境变量
				if len(cfg.Environment) > 0 {
					fmt.Printf("   ⚙️  Setting environment variables...\n")
					// 构建环境变量映射
					envMap := make(map[string]string)
					for _, env := range imgCfg.Config.Env {
						parts := strings.SplitN(env, "=", 2)
						if len(parts) == 2 {
							envMap[parts[0]] = parts[1]
						}
					}

					// 更新环境变量
					for k, v := range cfg.Environment {
						envMap[k] = v
					}

					// 转换回数组格式
					newEnv := make([]string, 0, len(envMap))
					for k, v := range envMap {
						newEnv = append(newEnv, fmt.Sprintf("%s=%s", k, v))
					}
					imgCfg.Config.Env = newEnv
				}

				// 12. 设置标签
				if len(cfg.Labels) > 0 {
					fmt.Printf("   ⚙️  Setting labels...\n")
					// 构建标签映射
					labelMap := make(map[string]string)
					for k, v := range imgCfg.Config.Labels {
						labelMap[k] = v
					}

					// 更新标签
					for k, v := range cfg.Labels {
						labelMap[k] = v
					}

					imgCfg.Config.Labels = labelMap
				}

				// 13. 设置卷
				if len(cfg.Volumes) > 0 {
					fmt.Printf("   ⚙️  Setting volumes...\n")
					// 构建卷映射
					volumeMap := make(map[string]struct{})
					for k := range imgCfg.Config.Volumes {
						volumeMap[k] = struct{}{}
					}

					// 更新卷
					for _, v := range cfg.Volumes {
						volumeMap[v] = struct{}{}
					}

					imgCfg.Config.Volumes = volumeMap
				}

				// 12. 设置命令
				if len(cfg.Cmd) > 0 {
					fmt.Printf("   ⚙️  Setting command: %v\n", cfg.Cmd)
					imgCfg.Config.Cmd = cfg.Cmd
				}

				// 13. 设置入口点
				if len(cfg.Entrypoint) > 0 {
					fmt.Printf("   ⚙️  Setting entrypoint: %v\n", cfg.Entrypoint)
					imgCfg.Config.Entrypoint = cfg.Entrypoint
				}

				// 14. 设置用户
				if cfg.User != "" {
					fmt.Printf("   ⚙️  Setting user: %s\n", cfg.User)
					imgCfg.Config.User = cfg.User
				}

				// 15. 设置工作目录
				if cfg.WorkingDir != "" {
					fmt.Printf("   ⚙️  Setting working directory: %s\n", cfg.WorkingDir)
					imgCfg.Config.WorkingDir = cfg.WorkingDir
				}

				// 16. 设置暴露端口
				if len(cfg.ExposedPorts) > 0 {
					fmt.Printf("   ⚙️  Setting exposed ports: %v\n", cfg.ExposedPorts)
					portMap := make(map[string]struct{})
					for _, port := range cfg.ExposedPorts {
						portMap[port] = struct{}{}
					}
					imgCfg.Config.ExposedPorts = portMap
				}

				// 17. 应用配置修改
				fmt.Printf("   📝 Applying config changes...\n")
				img, err = mutate.ConfigFile(img, imgCfg)
				if err != nil {
					return fmt.Errorf("mutating config: %w", err)
				}

				// 多个平台，保存到同一个 OCI layout 目录
				fmt.Printf("   💾 Saving image to OCI layout: %s\n", ociLayoutDir)
				if err := crane.SaveOCI(img, ociLayoutDir); err != nil {
					return fmt.Errorf("saving image to OCI layout: %w", err)
				}
				platformImageRefs = append(platformImageRefs, targetPlatform)

				fmt.Printf("   ✅ Platform %s completed!\n", targetPlatform)
			}

			repository := cfg.To.Repository
			var pushImage string
			for idex, tag := range cfg.To.Tags {

				if idex == 0 {
					pushImage = repository + ":" + tag

					fmt.Printf("\n📦 Creating multi-platform manifest...\n")
					fmt.Printf("   📤 Pushing to: %s\n", pushImage)

					// 从 OCI layout 加载索引
					fmt.Printf("   🛠️  Combining %d platform images from OCI layout...\n", len(platformImageRefs))
					idx, err := layout.ImageIndexFromPath(ociLayoutDir)
					if err != nil {
						return fmt.Errorf("loading OCI layout as index: %w", err)
					}
					// 根据 format 配置选择推送格式
					var pushIdx v1.ImageIndex = idx
					if strings.EqualFold(cfg.Format, "Docker") {
						fmt.Printf("   🔄 Converting to Docker Manifest List format...\n")
						pushIdx = mutate.IndexMediaType(idx, types.DockerManifestList)
					} else {
						fmt.Printf("   📋 Using OCI Image Index format...\n")
					}

					// 推送多平台镜像索引
					fmt.Printf("   📤 Pushing multi-platform image index...\n")
					o := crane.GetOptions(*options...)
					ref, err := name.ParseReference(pushImage, o.Name...)
					if err != nil {
						return fmt.Errorf("parsing reference: %w", err)
					}

					if err := remote.WriteIndex(ref, pushIdx, o.Remote...); err != nil {
						return fmt.Errorf("pushing multi-platform index: %w", err)
					}
				} else {
					nameImage := repository + ":" + tag
					fmt.Printf("   🏷️  Tagging as: %s\n", nameImage)
					// crane.Tag(src, tag) - src 是已存在的镜像，tag 是新标签
					if err := crane.Tag(pushImage, tag, *options...); err != nil {
						return fmt.Errorf("tagging image %s: %w", nameImage, err)
					}
				}

			}

			fmt.Printf("\n✅ Image creation completed successfully!\n")
			fmt.Printf("   🎉 Repository: %s\n", cfg.To.Repository)
			fmt.Printf("   🎉 Tags: %v\n", cfg.To.Tags)
			fmt.Printf("   🎉 Platforms: %v\n", platformImageRefs)
			return nil
		},
	}
	createCmd.Flags().StringVarP(&configFile, "config", "c", "", "Path to config file to use for creating the new image")
	createCmd.Flags().StringSliceVarP(&valFiles, "valf", "f", nil, "Path to variable file (yaml format) to inject into config")
	createCmd.Flags().Var(&vals, "val", "Dynamic variables in key=value format to inject into config")

	return createCmd
}
