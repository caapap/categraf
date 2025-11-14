# Git 操作指南

## 基于 Tag 创建二开分支

### 场景
从 fork 仓库基于源仓库的某个 tag（如 v0.4.9）创建二开分支。

### 操作步骤

1. **添加 upstream 远程仓库**
```bash
git remote add upstream https://github.com/flashcatcloud/categraf.git
```

2. **获取所有 tags**
```bash
git fetch upstream --tags
```

3. **删除旧的远程分支（如已存在）**
```bash
git push origin --delete stellar-categraf
```

4. **基于 tag 创建新分支**
```bash
git checkout -b stellar-categraf v0.4.9
```

5. **推送到远程**
```bash
git push -u origin stellar-categraf
```

### 验证
```bash
git describe --tags  # 应显示 v0.4.9
git branch -vv       # 查看分支跟踪状态
```

### 同步 upstream 更新
```bash
git fetch upstream
git merge upstream/main  # 或使用 cherry-pick 选择特定提交
```

---

## 合并二开内容到 Tag 分支

### 场景
已有基于最新 main 的二开代码，需要将二开内容（如新增插件）迁移到基于特定 tag 的分支。

### 操作步骤

假设：
- 源二开目录：`/path/to/categraf-new`（基于最新 main）
- 目标分支：`stellar-categraf`（基于 v0.4.9）
- 新增插件：`ollama` 和 `spark_streaming`

#### 1. 复制插件代码
```bash
# 切换到目标分支
cd /path/to/categraf
git checkout stellar-categraf

# 复制插件目录
cp -r /path/to/categraf-new/inputs/ollama ./inputs/
cp -r /path/to/categraf-new/inputs/spark_streaming ./inputs/

# 复制配置文件
cp -r /path/to/categraf-new/conf/input.ollama ./conf/
cp -r /path/to/categraf-new/conf/input.spark_streaming ./conf/
```

#### 2. 注册插件
编辑 `agent/metrics_agent.go`，在 import 区域按字母顺序添加：
```go
_ "flashcat.cloud/categraf/inputs/ollama"
_ "flashcat.cloud/categraf/inputs/spark_streaming"
```

#### 3. 更新依赖
```bash
go mod tidy
```

#### 4. 编译测试
```bash
go build -o categraf
```

#### 5. 提交并推送
```bash
# 配置 git 用户信息（如需要）
git config user.email "your@example.com"
git config user.name "Your Name"

# 添加所有更改
git add .

# 提交
git commit -m "feat: 添加 ollama 和 spark_streaming 插件

- 新增 ollama 插件，支持 Ollama API 监控和代理
- 新增 spark_streaming 插件，支持 Spark Streaming 应用监控
- 更新 metrics_agent.go 注册新插件
- 添加相应配置文件
- 更新依赖"

# 推送
git push origin stellar-categraf
```

### 关键点
- 插件代码放在 `inputs/插件名/` 目录
- 配置文件放在 `conf/input.插件名/` 目录
- 必须在 `agent/metrics_agent.go` 中注册插件
- 使用 `go mod tidy` 自动处理新增依赖
- 编译成功后再提交

---

## GitHub Actions 和 GoReleaser 配置

### 场景
配置自动化构建和发布流程，支持打 tag 时自动构建、发布 release 并推送 Docker 镜像。

### 操作步骤

#### 1. 配置 GoReleaser (.goreleaser.yaml)

**关键配置项**：

```yaml
release:
  github:
    owner: caapap  # 注意：使用你的 fork 仓库名
    name: categraf
  name_template: "v{{ .Version }}"
  mode: replace  # 替换已存在的 release，避免文件冲突
```

**构建配置**（示例：仅保留 Linux 标准版）：
```yaml
builds:
  - id: linux-amd64
    # ... 标准 Linux AMD64 构建
  - id: linux-arm64
    # ... 标准 Linux ARM64 构建
  - id: linux-amd64-cgo
    # ... 带 CGO 的构建（用于特殊插件）
```

**Docker 镜像配置**（华为云 SWR）：
```yaml
dockers:
  - image_templates:
      - swr.cn-east-3.myhuaweicloud.com/caapap/categraf:{{ .Tag }}-amd64
    ids:
      - linux-amd64
    # ...

docker_manifests:
  - name_template: swr.cn-east-3.myhuaweicloud.com/caapap/categraf:{{ .Tag }}
    image_templates:
      - swr.cn-east-3.myhuaweicloud.com/caapap/categraf:{{ .Tag }}-amd64
      - swr.cn-east-3.myhuaweicloud.com/caapap/categraf:{{ .Tag }}-arm64v8
```

#### 2. 配置 GitHub Actions Workflow (.github/workflows/release.yaml)

```yaml
name: Release

on:
  push:
    tags:
      - 'v*'
  workflow_dispatch:  # 支持手动触发

env:
  GO_VERSION: 1.22

jobs:
  goreleaser:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout Source Code
        uses: actions/checkout@v4
        with:
          fetch-depth: 0
      
      - name: Setup Go Environment
        uses: actions/setup-go@v5
        with:
          go-version: ${{ env.GO_VERSION }}
      
      - name: Login Huawei-Cloud SWR
        uses: docker/login-action@v3
        with:
          registry: swr.cn-east-3.myhuaweicloud.com
          username: ${{ secrets.HUAWEI_SWR_USERNAME }}
          password: ${{ secrets.HUAWEI_SWR_PASSWORD }}
      
      - name: Run GoReleaser
        uses: goreleaser/goreleaser-action@v5
        with:
          version: latest
          args: release --clean --timeout 60m
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

#### 3. 配置 GitHub Secrets

在 GitHub 仓库设置中添加以下 Secrets：

- `HUAWEI_SWR_USERNAME`: 华为云 SWR 用户名
- `HUAWEI_SWR_PASSWORD`: 华为云 SWR 密码
- `GITHUB_TOKEN`: 通常自动提供，无需手动配置

#### 4. 创建并推送 Tag

```bash
# 创建 tag
git tag v0.4.9-20251114

# 推送 tag（会自动触发 Actions）
git push origin v0.4.9-20251114
```

#### 5. 重新推送 Tag（如果 release 已存在）

如果遇到 `422 Validation Failed` 错误（文件已存在），需要先删除旧 release：

```bash
# 删除本地 tag
git tag -d v0.4.9-20251114

# 删除远程 tag
git push origin :refs/tags/v0.4.9-20251114

# 重新创建并推送
git tag v0.4.9-20251114
git push origin v0.4.9-20251114
```

或者在 `.goreleaser.yaml` 中添加 `mode: replace` 自动处理。

### 常见问题

#### 问题 1: Release 发布到错误的仓库

**症状**：GoReleaser 尝试发布到 `flashcatcloud/categraf` 而不是 `caapap/categraf`

**解决**：检查 `.goreleaser.yaml` 中的 `release.github.owner` 配置：
```yaml
release:
  github:
    owner: caapap  # 确保是你的 fork 仓库
    name: categraf
```

#### 问题 2: 文件已存在错误 (422 Validation Failed)

**症状**：`422 Validation Failed [{Resource:ReleaseAsset Field:name Code:already_exists Message:}]`

**解决**：
1. 在 `.goreleaser.yaml` 中添加 `mode: replace`
2. 或手动删除 GitHub 上的旧 release
3. 或使用新的 tag 名称

#### 问题 3: Docker 镜像推送失败

**症状**：镜像构建成功但推送失败

**排查**：
- 检查 `HUAWEI_SWR_USERNAME` 和 `HUAWEI_SWR_PASSWORD` 是否正确配置
- 检查镜像仓库地址是否正确
- 检查是否有推送权限

### 关键点

- **仓库配置**：确保 `release.github.owner` 指向正确的 fork 仓库
- **Release 模式**：使用 `mode: replace` 避免文件冲突
- **构建优化**：移除不需要的构建（如 slim、windows）可加快构建速度
- **镜像推送**：GoReleaser 会自动构建并推送 Docker 镜像到配置的仓库
- **多架构支持**：通过 `docker_manifests` 创建多架构镜像清单

