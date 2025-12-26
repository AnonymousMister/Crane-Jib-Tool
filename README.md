

# Crane-Jib-Tool 🚀

一个轻量级、声明式的容器镜像构建工具。无需 Docker 守护进程，通过 `crane` 实现类似 Google Jib 的文件分层打包与推送功能。支持 JSON 模板、INI 配置注入以及动态变量替换。

## ✨ 特性

* **免 Docker 构建**：直接生成 OCI 兼容镜像并推送到远程仓库。
* **声明式分层**：通过 `crane.json` 定义本地文件到镜像路径的精确映射。
* **动态模板引擎**：支持 `${Variable}` 占位符，可从环境、文件或命令行注入。
* **内置时间戳**：自动生成 `${TimestampTag}` 变量，支持版本回溯。
* **多格式支持**：变量源支持 `.json`、`.ini` 文件及 `KEY=VALUE` 字符串。

## 📦 安装

首先，确保您的项目中已安装必要的依赖：

```bash
npm install tar ini
# 或者
pnpm add tar ini

```

## 🛠 快速开始

### 1. 准备镜像模板 `crane.json`

在项目根目录创建模板文件，使用占位符定义动态内容：

```json
{
  "from": "nginx:stable-alpine",
  "image": "swr.cn-east-3.myhuaweicloud.com/your-project/${APP_NAME}",
  "tag": "${TimestampTag}",
  "exposedPorts": [80, 443],
  "envs": {
    "APP_ENV": "${NODE_ENV}",
    "BUILD_VERSION": "${VERSION}"
  },
  "layers": [
    {
      "name": "nginx-conf",
      "files": [
        { "from": "./nginx.conf", "to": "etc/nginx/conf.d/default.conf" }
      ]
    },
    {
      "name": "static-assets",
      "files": [
        { "from": "./dist", "to": "usr/share/nginx/html" }
      ]
    }
  ]
}

```

### 2. 执行构建

使用 `-t` 指定模板，使用 `-f` 注入变量：

```bash
# 基础用法（使用内置时间戳）
node bin/build.js -t crane.json -f "APP_NAME=my-admin" -f "NODE_ENV=prod"

# 覆盖自动生成的 Tag
node bin/build.js -t crane.json -f "TimestampTag=v1.0.0" -f "APP_NAME=web"

```

## 📖 详细用法

### 变量注入优先级

工具会按以下顺序合并变量（后者覆盖前者）：

1. **系统环境变量** (`process.env`)。
2. **内置变量**：`${TimestampTag}` (格式: `YYYYMMDDHHMMSS`)。
3. **文件变量**：通过 `-f config.json` 或 `-f config.ini` 加载。
4. **命令行变量**：通过 `-f "KEY=VALUE"` 直接指定。

### 使用 INI 文件作为变量源

您可以创建一个 `deploy.ini`：

```ini
APP_NAME = wlhy-wj-admin
VERSION = 2.4.5
DOCKER_USER = your_username
DOCKER_PASS = your_password

```

然后运行：

```bash
node bin/build.js -t crane.json -f deploy.ini

```

## 🚀 运行流程说明

1. **变量初始化**：收集环境、内置时间戳及 `-f` 传入的所有参数。
2. **模板渲染**：将 `crane.json` 中的占位符替换为实际数值。
3. **本地打包**：根据 `layers` 定义，将本地文件临时打包为 `tar` 层。
4. **推送镜像**：调用 `crane append` 将所有层合并推送到目标仓库。
5. **修改元数据**：调用 `crane mutate` 配置 `ExposedPorts` 和 `Env` 变量。
6. **自动清理**：构建完成后自动删除 `.crane_tmp` 临时目录。

## ⚠️ 注意事项

* **认证**：如果提供了 `DOCKER_USER` 和 `DOCKER_PASS` 变量，工具会自动执行 `crane auth login`。
* **安全性**：对于私有仓库（如 `192.168.x.x`），工具会自动添加 `--insecure` 标志。
