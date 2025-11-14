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

