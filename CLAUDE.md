# CLAUDE.md

此文件为 Claude Code (claude.ai/code) 使用此代码库中的代码提供指导。

## 项目概述

这是一个飞书 (Feishu) 机器人项目，目前处于初始开发阶段。该项目旨在为飞书平台创建一个机器人并使用机器完成类似 Claude code 的远程开发能力

项目的开发实现方式需要参考 [Claude-Code-Remote-Analysis.md](Claude-Code-Remote-Analysis.md)


## 当前状态

该项目非常精简，包含：
- 包含飞书应用凭据和卡片模板 ID 的配置文件
- 尚未实现源代码
- 尚未配置构建系统或依赖项

## 配置

### 飞书应用配置
应用配置存储在 `configs/config.yaml` 中：
- 应用 ID：`cli_a8058428d478501c`
- 应用密钥：可在 config 中找到（应移至环境变量）
- 基础域名：`https://open.feishu.cn`

### 卡片模板
已配置三个卡片模板：
- 停止任务卡片：`AAqz1Y1TvQB25`
- 正在运行的任务卡片：`AAqz1Y1p8y5Se`
- 成功任务卡片：`AAqz1Y1QyEzLF`

## 开发设置

由于这是一个空项目，您需要：

1. 初始化 go 项目：
```bash

```

2. 安装飞书 SDK 及依赖项（典型选择）：


3. 将敏感配置移至环境变量：
- 为 app_secret 和其他凭证创建 `.env` 文件
- 更新 config.yaml 文件以移除敏感数据

## 架构考量

根据卡片模板配置，此机器人可能处理：
- claude code hooks 传递过来的任务状态通知
- 飞书中的交互式卡片消息
- 使用websocket 和飞书进行连接

实施时，请考虑
- 飞书事件的 Webhook 处理
- 消息卡片渲染和交互
- 任务状态管理
- 错误处理和日志记录

## 安全注意事项

- **重要**：应立即将 `config.yaml` 中的 app_secret 移至环境变量
- 实施适当的 Webhook 签名验证
-为所有用户输入添加输入验证
- 所有 webhook 端点均使用 HTTPS