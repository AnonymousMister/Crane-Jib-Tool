

# crane-jib-tool 🚀

一个轻量级、声明式的容器镜像构建工具。无需 Docker 守护进程，通过 `crane-jib-tool` 实现类似 Google Jib 的文件分层打包与推送功能。支持 JSON 模板、INI 配置注入以及动态变量替换。

## ✨ 特性

* **免 Docker 构建**：直接生成 OCI 兼容镜像并推送到远程仓库。
* **声明式分层**：通过 `crane.yaml` 定义本地文件到镜像路径的精确映射。
* **动态模板引擎**：支持 `${Variable}` 占位符，可从环境、文件或命令行注入。
* **内置时间戳**：自动生成 `${TimestampTag}` 变量，支持版本回溯。
* **多格式支持**：变量源支持 `.yaml` 文件及 `KEY=VALUE` 字符串。
* **npm 包集成**：方便集成到 npm 项目中，支持 `npx` 调用。

## 📦 安装

### 全局安装

```bash
npm install -g @anonymousmister/crane-jib-tool
# 或者
pnpm add -g @anonymousmister/crane-jib-tool
```

### 项目内安装

```bash
npm install @anonymousmister/crane-jib-tool --save-dev
# 或者
pnpm add @anonymousmister/crane-jib-tool -D
```

## 🛠 快速开始

### 1. 准备镜像模板 `crane.yaml`

在项目根目录创建模板文件，使用占位符定义动态内容：


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

#### 使用全局安装的命令

```bash
# 基础用法（使用内置时间戳）
crane-jib-tool create -c configFile.yaml  

# 覆盖自动生成的 Tag
crane-jib-tool create -c configFile.yaml  
```

#### 使用 npx 调用（项目内安装）

```bash
# 基础用法（使用内置时间戳）
npx crane-jib-tool create -c configFile.yaml  

# 覆盖自动生成的 Tag
npx crane-jib-tool create -c configFile.yaml  
```

### 3. 手动安装二进制文件

如果您需要手动安装 `crane-jib-tool` 二进制文件，可以使用以下命令：

```bash
# 全局安装
crane-jib-tool  install

# 项目内安装
npx crane-jib-tool install
```

## 📖 详细用法

### 命令列表

| 命令                           | 描述 |
|------------------------------| --- |
| `crane install`              | 安装 `crane-jib-tool` 二进制文件 |
| `crane create -c <template>` | 执行镜像构建 |
| `crane help`                 | 显示帮助信息 |
| 其他命令                         | 直接转发给 `crane-jib-tool` 二进制文件执行 |

### 变量注入优先级

工具会按以下顺序合并变量（后者覆盖前者）：

1. **系统环境变量** (`process.env`)。
2. **内置变量**：`${TimestampTag}` (格式: `YYYYMMDDHHMMSS`)。
3. **文件变量**：通过 `-f config.yaml` 加载。
4. **命令行变量**：通过 `--val "KEY=VALUE"` 直接指定。

### 使用 INI 文件作为变量源

您可以创建一个 `config.yaml`：

```ini
APP_NAME = wlhy-wj-admin
VERSION = 2.4.5
DOCKER_USER = your_username
DOCKER_PASS = your_password
```

然后运行：

```bash
crane-jib-tool create -c configFile.yaml   -f config.yaml
```

### 支持的模板格式

除了基本的 JSON 模板格式外，`crane-jib-tool` 还支持 YAML 格式的模板文件，例如：

```yaml
apiVersion: jib/v1alpha1
kind: BuildFile

from:
  image: nginx:1.13.5
  platforms:
    - "linux/amd64"
    - "linux/arm/v7"
to: swr.cn-east-3.myhuaweicloud.com/your-project/${APP_NAME}:${TimestampTag}

format: Docker

exposedPorts:
  - "80"
  - "443"

layers:
  entries:
    - name: "nginx-conf"
      files:
        - src: "./nginx.conf"
          dest: "/etc/nginx/conf.d/default.conf"
    - name: "static-assets"
      files:
        - src: "./dist"
          dest: "/usr/share/nginx/html"
```

## 🚀 运行流程说明

1. **变量初始化**：收集环境、内置时间戳及 `-f` 传入的所有参数。
2. **模板渲染**：将  `crane.yaml` 中的占位符替换为实际数值。
3. **本地打包**：根据 `layers` 定义，将本地文件临时打包为 `tar` 层。
4. **合并层**：调用 `layers` 将所有层合并推送到目标仓库。
5. **修改元数据**：调用 `mutate` 配置 `ExposedPorts` 和 `Env` 变量。
6. **自动清理**：构建完成后自动删除临时目录。

 
## 📁 目录结构

```
├── bin/
│   ├── cli.js              # npm 包入口文件
│   └── install.js          # 二进制文件安装脚本
├── examples/               # 示例配置文件
│   ├── configFile.yaml     # 完整的 YAML 配置示例
│   ├── example-config-1.yaml # 简单的 YAML 配置示例
│   └── examples.conf       # Nginx 配置示例
├── package.json            # npm 包配置
└── README.md               # 项目文档
```

