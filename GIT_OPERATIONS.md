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

