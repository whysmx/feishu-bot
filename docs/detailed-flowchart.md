# 飞书机器人详细流程图补充

本文档补充当前实现的关键流程：Claude CLI 启动、流式分段发送、命令处理与会话管理。

## 1. Claude CLI 启动与管理流程

```mermaid
flowchart TD
    Start([收到用户消息]) --> Init[初始化 ClaudeManager]
    Init --> SetupCmd[设置命令参数]
    SetupCmd --> CmdParams["-p --output-format stream-json --include-partial-messages --verbose"]

    CmdParams --> CreateCmd[exec.Command claude]
    CreateCmd --> Env[注入 ANTHROPIC_* 与 Claude Code 环境变量]
    Env --> SetupStdin[设置 stdin/stdout/stderr]
    SetupStdin --> StartProcess[启动进程]

    StartProcess --> WriteInput[写入用户消息]
    WriteInput --> CloseStdin[关闭 stdin 发送 EOF]
    CloseStdin --> StartReader[启动 stdout 读取协程]

    StartReader --> ReadLoop{读取输出}
    ReadLoop -->|有数据| ParseLine[解析 JSON 行]
    ReadLoop -->|EOF/错误| ExitLoop

    ParseLine --> ValidateJSON{有效 JSON?}
    ValidateJSON -->|否| ReadLoop
    ValidateJSON -->|是| ExtractEvent[提取事件类型]

    ExtractEvent --> EventType{事件类型}
    EventType -->|content_block_delta| Callback[触发文本增量回调]
    EventType -->|assistant/system| SyncState[同步会话状态]
    EventType -->|message_stop| ExitLoop
    EventType -->|其他| ReadLoop

    ExitLoop --> Cleanup[清理资源]
    Cleanup --> WaitProcess[等待进程结束]
    WaitProcess --> End([结束])

    style Start fill:#e1f5e1
    style End fill:#e1f5e1
    style StartProcess fill:#ffe1f5
```

## 2. 流式分段发送流程

```mermaid
flowchart TD
    Start([收到文本增量]) --> Append[追加到缓冲区]
    Append --> CheckSize{缓冲区超过上限?}

    CheckSize -->|是| ForceSend[强制分段发送]
    CheckSize -->|否| StartTimers[启动空闲/持续时间定时器]

    StartTimers --> IdleTimer{空闲超时?}
    IdleTimer -->|是| SendIdle[发送缓冲区]
    IdleTimer -->|否| DurationTimer{持续时间超时?}

    DurationTimer -->|是| SendDuration[发送缓冲区]
    DurationTimer -->|否| WaitMore[继续累积]

    SendIdle --> WaitMore
    SendDuration --> WaitMore
    ForceSend --> WaitMore

    WaitMore --> End([等待下一批增量])

    style Start fill:#e1f5e1
    style End fill:#e1f5e1
```

**分段触发条件**：
- `StreamIdleTimeout`：空闲一段时间后发送
- `StreamMaxDuration`：持续输出超过阈值后发送
- `StreamMaxBufferSize`：缓冲区超过最大字符数时发送

## 3. 消息解析与命令处理流程

```mermaid
flowchart TD
    Start([收到飞书事件]) --> ParseEvent[解析事件结构]
    ParseEvent --> CheckType{事件类型?}

    CheckType -->|非消息事件| Ignore1([忽略])
    CheckType -->|im.message.receive_v1| ExtractMsg[提取消息内容]

    ExtractMsg --> ParseContent[解析 content JSON]
    ParseContent --> ContentCheck{内容格式?}

    ContentCheck -->|非 text 类型| Ignore2([忽略])
    ContentCheck -->|text 类型| ExtractText[提取文本]

    ExtractText --> ChatType{chat_type?}

    ChatType -->|p2p| P2PMode[P2P处理]
    ChatType -->|group/private| GroupMode[群聊处理]

    GroupMode --> Mention{是否@机器人?}
    Mention -->|否| ToClaude[直接转发给 Claude]
    Mention -->|是| CommandCheck{是否命令?}

    CommandCheck -->|是| CommandHandle[执行 ls/bind/help]
    CommandCheck -->|否| StripMention[移除@前缀]
    StripMention --> ToClaude

    P2PMode --> ToClaude
    ToClaude --> End([进入流式对话流程])

    style Start fill:#e1f5e1
    style End fill:#e1f5e1
```

## 4. 会话管理与恢复

- **P2P**：按用户 `open_id/union_id` 维护会话，重复对话会使用 `--resume`。
- **群聊**：使用全局会话 ID（`global_group_session`），所有群聊共享上下文。
- **恢复失败**：若 CLI 返回 "No conversation found"，会自动重试一次（不带 `--resume`）。

## 5. 绑定配置与基础目录

- 绑定关系存储在 `configs/chat_config.json`。
- `BASE_DIR` 或配置文件中的 `base_dir` 决定 `ls/bind` 的扫描目录。
- `bind <序号>` 将群聊绑定到对应项目路径，后续 Claude CLI 在该目录下运行。
