# Crane-Jib-Tool 🚀

一个轻量级、声明式的容器镜像构建工具，无需 Docker 守护进程，通过 `crane` 实现类似 Google Jib 的文件分层打包与推送功能。支持多种配置格式、动态变量替换和跨平台构建。

## ✨ 特性

### 核心功能
- **免 Docker 构建**：直接生成 OCI 兼容镜像并推送到远程仓库
- **声明式分层**：通过配置文件定义本地文件到镜像路径的精确映射
- **多平台支持**：支持 Linux/amd64、Linux/arm、Darwin/amd64 等多种平台
- **动态模板引擎**：支持 `${Variable}` 占位符，可从环境、文件或命令行注入
- **内置时间戳**：自动生成 `${TimestampTag}` 变量，支持版本回溯
- **多格式支持**：变量源支持 `.yaml` 文件及 `KEY=VALUE` 字符串

### 高级特性
- **OCI Layout 支持**：使用 OCI layout 存储多平台镜像
- **跨平台兼容**：在 Windows、Linux 和 macOS 上均可运行
- **自定义权限**：支持为文件和目录设置自定义权限
- **文件过滤**：支持 `excludes` 和 `includes` 规则
- **创建时间设置**：支持自定义镜像创建时间

## 📦 安装

### 使用 Go 命令行工具

```bash
# 克隆仓库
git clone https://github.com/AnonymousMister/crane-jib-tool.git
cd crane-jib-tool

# 构建二进制文件
go build -o crane-jib-tool main.go

# 或者直接使用 go install
go install github.com/AnonymousMister/crane-jib-tool
```

## 🛠 快速开始

### 1. 准备配置文件 `configFile.yaml`

```yaml
apiVersion: jib/v1alpha1
kind: BuildFile

from:
  image: "ubuntu"
  platforms:
    - "linux/amd64"
    - architecture: "arm"
      os: "linux"

creationTime: 2000
format: Docker
environment:
  "KEY1": "v1"
  "KEY2": "v2"

labels:
  "label1": "l1"
  "label2": "l2"

volumes:
  - "/volume1"
  - "/volume2"
exposedPorts:
  - "123/udp"
  - "456"
  - "789/tcp"

user: "customUser"
workingDirectory: "/home"

entrypoint:
  - "sh"
  - "script.sh"
cmd:
  - "--param"
  - "param"

layers:
  properties:
    filePermissions: "644"
    directoryPermissions: "755"
    user: "0"
    group: "0"
    timestamp: "1000"
  entries:
    - name: "scripts"
      properties:
        filePermissions: "755"
      files:
        - src: "project/run.sh"
          dest: "/home/run.sh"
        - src: "scripts"
          dest: "/home/scripts"
          excludes:
            - "**/exclude.me"
            - "**/*.ignore"
          includes:
            - "**/include.me"
```

### 2. 执行构建

```bash
# 使用配置文件构建镜像
crane-jib-tool create -c configFile.yaml

# 传入变量
crane-jib-tool create -c configFile.yaml --valf vars.yaml --val KEY=VALUE
```

## 📖 详细用法

### 命令说明

#### `crane-jib-tool create`

**功能**：创建并推送容器镜像

**参数**：
- `-c, --config string`：必填，配置文件路径
- `-v, --val stringArray`：可选，直接传入变量，格式 `KEY=VALUE`
- `-f, --valf stringArray`：可选，从文件加载变量，支持 `.json`、`.ini` 格式
- `--insecure`：可选，允许访问不安全的仓库

**示例**：
```bash
# 基础用法
crane-jib-tool create -c config.yaml

# 带文件变量
crane-jib-tool create -c config.yaml --valf vars.json

# 带命令行变量
crane-jib-tool create -c config.yaml --val APP_VERSION=1.0.0 --val ENV=production

# 混合变量源
crane-jib-tool create -c config.yaml --valf vars.yaml --val DEBUG=true
```

### 配置模板结构

配置文件采用 YAML 格式，支持以下核心字段：

```yaml
# 版本和类型（固定）
apiVersion: jib/v1alpha1
kind: BuildFile

# 基础镜像配置
from:
  image: "基础镜像名称"  # 例如：ubuntu:20.04
  platforms:           # 支持的平台列表
    - "linux/amd64"    # 字符串格式
    - architecture: "arm"  # 结构化格式
      os: "linux"

# 镜像元数据
creationTime: 2000     # 镜像创建时间戳
format: Docker         # 镜像格式

environment:           # 环境变量
  "KEY1": "value1"
  "KEY2": "value2"

labels:                # 镜像标签
  "label1": "value1"

volumes:               # 卷挂载点
  - "/volume1"

exposedPorts:          # 暴露端口
  - "80/tcp"

user: "customUser"     # 容器运行用户
workingDirectory: "/home"  # 工作目录

entrypoint:            # 入口命令
  - "sh"
  - "script.sh"
cmd:                   # 容器参数
  - "--param"
  - "value"

# 分层配置
layers:
  properties:          # 全局层属性
    filePermissions: "644"        # 文件权限
    directoryPermissions: "755"   # 目录权限
    user: "0"                     # 用户 ID
    group: "0"                    # 组 ID
    timestamp: "1000"             # 文件时间戳
  entries:             # 层列表
    - name: "layer-name"          # 层名称
      properties:                  # 层特定属性（覆盖全局属性）
        filePermissions: "755"
      files:                      # 文件映射列表
        - src: "local/path"       # 本地文件/目录路径
          dest: "/container/path" # 容器内路径
          excludes:               # 排除规则
            - "**/*.log"
          includes:               # 包含规则
            - "**/*.txt"
```

### 变量注入机制

#### 变量优先级

工具会按以下顺序合并变量（后者覆盖前者）：

1. **系统环境变量**：自动读取当前环境中的变量
2. **内置变量**：
   - `${TimestampTag}`：自动生成，格式为 `YYYYMMDDHHMMSS`
3. **文件变量**：通过 `--valf` 加载的变量文件
4. **命令行变量**：通过 `--val` 直接指定的变量

#### 变量注入示例

1. **配置文件中使用变量**

```yaml
from:
  image: "ubuntu:${UBUNTU_VERSION}"

labels:
  "version": "${APP_VERSION}"
  "build-time": "${TimestampTag}"
```

2. **从环境变量注入**

```bash
# 先设置环境变量
export UBUNTU_VERSION=22.04
export APP_VERSION=1.0.0

# 执行命令时自动读取环境变量
crane-jib-tool create -c config.yaml
```

3. **从文件注入变量**

创建 `vars.yaml` 文件：
```yaml
UBUNTU_VERSION: 22.04
APP_VERSION: 1.0.0
```

执行命令：
```bash
crane-jib-tool create -c config.yaml --valf vars.yaml
```

4. **从命令行注入变量**

```bash
crane-jib-tool create -c config.yaml --val UBUNTU_VERSION=22.04 --val APP_VERSION=1.0.0
```

5. **混合注入方式**

```bash
export UBUNTU_VERSION=22.04
crane-jib-tool create -c config.yaml --valf vars.yaml --val APP_VERSION=2.0.0
```

## 🚀 运行流程

1. **变量初始化**：收集环境、内置时间戳及命令行传入的所有参数
2. **配置解析**：解析配置文件并应用变量替换
3. **平台处理**：处理多平台配置，生成平台列表
4. **层准备**：为每个层创建临时目录，复制文件并应用过滤规则
5. **层打包**：将每个层打包为 tar 文件
6. **镜像构建**：
   - 对于单个平台，直接推送镜像
   - 对于多个平台，使用 OCI layout 存储镜像，然后构建并推送多平台索引
7. **清理临时文件**：构建完成后自动清理临时目录

## ⚠️ 注意事项

1. **认证**：确保已登录到目标容器 registry
2. **私有仓库**：对于私有仓库，使用 `--insecure` 标志
3. **路径处理**：
   - 如果 src 是目录，dest 始终视为目录
   - 如果 src 是文件，dest 以 `/` 结尾视为目录，否则视为文件
4. **权限规则**：
   - 对于出现在多个层级的文件或目录，其权限将优先考虑文件所在的最后一层级
   - 未显式定义的父目录使用默认权限（755）
5. **跨平台构建**：确保在构建平台上安装了相应的交叉编译工具链

## 📁 项目结构

```
crane-jib-tool/
├── cmd/              # Go 命令行工具实现
│   ├── auth.go       # 认证相关代码
│   ├── create.go     # 镜像创建核心逻辑
│   ├── root.go       # 命令行根命令
│   └── util.go       # 工具函数
├── examples/         # 示例配置文件
│   ├── configFile.yaml
│   └── example-config-1.yaml
├── pkg/              # 核心包
│   ├── config/       # 配置解析
│   ├── layer/        # 层处理
│   └── tarutil/      # 跨平台 tar 工具
├── .goreleaser.yaml  # goreleaser 配置
├── README.md         # 项目文档
├── go.mod            # Go 模块配置
└── main.go           # 主入口
```

