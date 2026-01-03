# 飞书机器人详细流程图补充

## 1. Claude CLI 启动与管理流程

```mermaid
flowchart TD
    Start([收到用户消息]) --> Init[初始化 ClaudeManager]
    Init --> SetupCmd[设置命令参数]
    SetupCmd --> CmdParams["-p (非交互模式)<br/>--output-format stream-json<br/>--include-partial-messages"]

    CmdParams --> CreateCmd[exec.Command cc1]
    CreateCmd --> SetupStdin[设置标准输入管道]
    SetupStdin --> SetupStdout[设置标准输出管道]
    SetupStdout --> StartProcess[启动进程]

    StartProcess --> WriteInput[写入用户消息到 stdin]
    WriteInput --> StartReader[启动 stdout 读取协程]

    StartReader --> ReadLoop{读取输出}
    ReadLoop -->|有数据| ParseLine[解析 JSON 行]
    ReadLoop -->|EOF/错误| ExitLoop

    ParseLine --> ValidateJSON{有效 JSON?}
    ValidateJSON -->|否| ReadLoop
    ValidateJSON -->|是| ExtractEvent[提取事件类型]

    ExtractEvent --> EventType{事件类型}
    EventType -->|content_block_delta| Callback[触发回调]
    EventType -->|message_stop| ExitLoop
    EventType -->|其他| ReadLoop

    Callback --> Accumulate[累积文本]
    Accumulate --> CheckLimit{超过阈值?}
    CheckLimit -->|是| TriggerUpdate[触发更新]
    CheckLimit -->|否| ReadLoop
    TriggerUpdate --> ReadLoop

    ExitLoop --> Cleanup[清理资源]
    Cleanup --> CloseStdin[关闭 stdin 管道]
    CloseStdin --> WaitProcess[等待进程结束]
    WaitProcess --> KillProcess[强制终止进程]
    KillProcess --> End([结束])

    style Start fill:#e1f5e1
    style End fill:#e1f5e1
    style StartProcess fill:#ffe1f5
    style KillProcess fill:#ffe1e1
```

## 2. CardKit 流式更新详细流程

```mermaid
flowchart TD
    Start([收到文本增量]) --> Append[追加到累积缓冲区]
    Append --> CheckRate{距离上次更新<br/>时间 >= 100ms?}

    CheckRate -->|否| Wait[等待]
    Wait --> CheckRate
    CheckRate -->|是| PrepareUpdate[准备更新请求]

    PrepareUpdate --> BuildElement[构建卡片元素]
    BuildElement --> MarkdownElement["element:<br/>  tag: lark_md<br/>  content: {text: 累积文本}"]

    MarkdownElement --> APIRequest[调用 CardKit 更新 API]
    APIRequest --> HTTPTarget["PUT /open-apis/cardkit/v1/cards/<br/>{card_id}/elements/{element_id}/content"]

    HTTPTarget --> SendRequest[发送 HTTP 请求]
    SendRequest --> Response{响应状态}

    Response -->|成功 200| RecordTime[记录更新时间]
    Response -->|限流 429| Backoff[退避重试]
    Response -->|其他错误| LogError[记录错误]

    Backoff --> WaitRetry[等待 500ms]
    WaitRetry --> APIRequest

    RecordTime --> CheckEnd{响应完成?}
    CheckEnd -->|否| Append
    CheckEnd -->|是| End([结束])

    LogError --> End

    style Start fill:#e1f5e1
    style End fill:#e1f5e1
    style APIRequest fill:#fff4e1
    style Backoff fill:#ffe1e1
```

## 3. 消息解析与验证流程

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

    ExtractText --> CheckChat{Chat 字段?}

    CheckChat -->|有值| GroupMode[群聊模式]
    CheckChat -->|无值| P2PMode[单聊模式]

    GroupMode --> ExtractChatID[提取 chat_id]
    ExtractChatID --> ValidateMention{是否@机器人?}

    ValidateMention -->|否| Ignore3([忽略])
    ValidateMention -->|是| GroupSuccess[群聊处理成功]

    P2PMode --> ExtractOpenID[提取 open_id]
    ExtractOpenID --> P2PSuccess[单聊处理成功]

    GroupSuccess --> SetParams["receive_id_type=chat_id<br/>receive_id=群聊ID"]
    P2PSuccess --> SetParams2["receive_id_type=open_id<br/>receive_id=用户open_id"]

    SetParams --> Next[进入消息处理流程]
    SetParams2 --> Next

    style Start fill:#e1f5e1
    style Ignore1 fill:#ffe1e1
    style Ignore2 fill:#ffe1e1
    style Ignore3 fill:#ffe1e1
    style GroupSuccess fill:#e1e5ff
    style P2PSuccess fill:#e1e5ff
    style Next fill:#e1f5e1
```

## 4. 错误处理与重试机制

```mermaid
flowchart TD
    Start([操作执行]) --> Execute[执行操作]
    Execute --> Result{操作结果}

    Result -->|成功| End([成功结束])

    Result -->|失败| ErrorClassify{错误类型?}

    ErrorClassify -->|网络错误| Network[网络错误处理]
    ErrorClassify -->|API 错误| API[API 错误处理]
    ErrorClassify -->|超时| Timeout[超时处理]

    Network --> CheckRetry{重试次数 < 3?}
    CheckRetry -->|是| WaitNet[等待 1s]
    WaitNet --> Execute

    API --> CheckCode{错误码?}
    CheckCode -->|230002| Fix23002[修复 receive_id]
    CheckCode -->|429 限流| Backoff[指数退避]
    CheckCode -->|其他| LogAPI[记录日志]

    Fix23002 --> ValidateFix{验证修复}
    ValidateFix -->|成功| Execute
    ValidateFix -->|失败| LogAPI

    Backoff --> CalcDelay["延迟 = 2^retry * 100ms"]
    CalcDelay --> WaitDelay[等待]
    WaitDelay --> Execute

    Timeout --> CheckTimeout{超时次数 < 2?}
    CheckTimeout -->|是| IncreaseTimeout[增加超时时间]
    IncreaseTimeout --> Execute
    CheckTimeout -->|否| LogTimeout[记录超时日志]

    LogAPI --> Fail([失败结束])
    LogTimeout --> Fail

    style Start fill:#e1f5e1
    style End fill:#e1f5e1
    style Fail fill:#ffe1e1
    style Fix23002 fill:#fff4e1
```

## 5. WebSocket 连接管理流程

```mermaid
flowchart TD
    Start([启动机器人]) --> InitWS[初始化 WebSocket 客户端]
    InitWS --> SetupHandler[设置事件处理器]
    SetupHandler --> Subscribe[订阅 im.message.receive_v1]

    Subscribe --> Connect[连接飞书 WebSocket]
    Connect --> Connected{连接成功?}

    Connected -->|失败| RetryConnect{重试次数 < 5?}
    RetryConnect -->|是| WaitConn[等待 5s]
    WaitConn --> Connect
    RetryConnect -->|否| FatalExit[退出程序]

    Connected -->|成功| Listen[开始监听事件]
    Listen --> EventLoop{收到事件?}

    EventLoop -->|是| Dispatch[分发事件]
    EventLoop -->|心跳| Pong[发送 Pong]
    EventLoop -->|断开| Reconnect[准备重连]

    Dispatch --> ValidateEvent{验证签名?}
    ValidateEvent -->|失败| LogInvalid[记录无效事件]
    ValidateEvent -->|成功| ProcessEvent[处理事件]

    ProcessEvent --> EventLoop

    Pong --> EventLoop
    LogInvalid --> EventLoop

    Reconnect --> WaitReconnect[等待 3s]
    WaitReconnect --> Connect

    style Start fill:#e1f5e1
    style FatalExit fill:#ffe1e1
    style Connected fill:#e1e5ff
    style Listen fill:#e1f5e1
```

## 6. 数据结构与消息流

```mermaid
graph LR
    subgraph 飞书事件
        A1[im.message.receive_v1]
        A2[event.sender.sender_id.open_id]
        A3[event.chat.id]
        A4[event.message.content]
    end

    subgraph 解析层
        B1[消息类型判断]
        B2[@机器人检测]
        B3[文本提取]
    end

    subgraph 处理层
        C1[receive_id]
        C2[receive_id_type]
        C3[用户消息]
    end

    subgraph Claude层
        D1[cc1 进程]
        D2[stream-json 输出]
        D3[文本增量]
    end

    subgraph CardKit层
        E1[card_id]
        E2[element_id]
        E3[更新 API]
    end

    A1 --> B1
    A2 --> B1
    A3 --> B1
    A4 --> B3

    B1 --> C1
    B2 --> C2
    B3 --> C3

    C3 --> D1
    D1 --> D2
    D2 --> D3

    D3 --> E3
    E3 --> E1
    E3 --> E2

    style A1 fill:#e1e5ff
    style D1 fill:#ffe1f5
    style E3 fill:#fff4e1
```

## 7. 并发模型与协程管理

```mermaid
flowchart TD
    Start([主程序启动]) --> MainGoroutine[主协程]

    MainGoroutine --> WSListen[WebSocket 监听协程]
    MainGoroutine --> EventQueue[事件处理协程池]
    MainGoroutine --> ClaudeManager[Claude 进程管理]

    WSListen --> ReceiveEvent[接收事件]
    ReceiveEvent --> SendToQueue[发送到队列]

    EventQueue --> Worker1[Worker 1]
    EventQueue --> Worker2[Worker 2]
    EventQueue --> Worker3[Worker 3]

    Worker1 --> ParseMsg[解析消息]
    Worker2 --> ParseMsg
    Worker3 --> ParseMsg

    ParseMsg --> Validation{验证通过?}
    Validation -->|是| StartClaude[启动 Claude CLI]
    Validation -->|否| Discard[丢弃]

    StartClaude --> StreamRead[流式读取协程]
    StreamRead --> CardUpdate[卡片更新协程]

    CardUpdate --> RateLimit[限流控制]
    RateLimit --> APIUpdate[API 更新]

    APIUpdate --> NextMsg{下一个增量?}
    NextMsg -->|是| StreamRead
    NextMsg -->|否| Cleanup[清理资源]

    Cleanup --> Worker1

    style Start fill:#e1f5e1
    style WSListen fill:#e1e5ff
    style Worker1 fill:#fff4e1
    style Worker2 fill:#fff4e1
    style Worker3 fill:#fff4e1
    style StreamRead fill:#ffe1f5
```

## 8. 关键状态机

### 8.1 消息处理状态机

```mermaid
stateDiagram-v2
    [*] --> Received: 收到飞书事件
    Received --> Validating: 验证格式
    Validating --> P2P: 单聊模式
    Validating --> Group: 群聊模式
    Validating --> Invalid: 格式错误

    P2P --> CheckMention: 检查@机器人
    Group --> CheckMention: 必须@

    CheckMention --> Processing: 通过验证
    CheckMention --> Invalid: 未@

    Processing --> CreateCard: 创建卡片
    CreateCard --> StreamClaude: 启动 Claude

    StreamClaude --> Streaming: 流式更新中
    Streaming --> StreamClaude: 继续接收
    Streaming --> Completed: 响应完成

    Completed --> [*]
    Invalid --> [*]
```

### 8.2 Claude CLI 状态机

```mermaid
stateDiagram-v2
    [*] --> Idle: 初始状态

    Idle --> Starting: 收到消息
    Starting --> Running: 进程启动成功
    Starting --> Error: 启动失败

    Running --> Streaming: 接收 stream-json
    Streaming --> Running: 继续接收
    Streaming --> Stopping: 收到 message_stop

    Stopping --> Cleanup: 清理资源
    Cleanup --> Idle: 完成

    Error --> Idle: 记录错误

    note right of Streaming
        累积文本
        触发更新
        限流控制
    end note
```

## 9. 性能瓶颈与优化点

```mermaid
flowchart LR
    subgraph 潜在瓶颈
        A1[Claude CLI 启动时间<br/>~500ms]
        A2[飞书 API 调用延迟<br/>~100ms]
        A3[流式更新频率<br/>受限于 10 QPS]
    end

    subgraph 优化措施
        B1[进程池预热]
        B2[批量更新合并]
        B3[客户端限流优化]
    end

    subgraph 优化后效果
        C1[启动延迟 ↓ 80%]
        C2[API 调用 ↓ 50%]
        C3[用户体验 ↑ 显著]
    end

    A1 -.优化.-> B1
    A2 -.优化.-> B2
    A3 -.优化.-> B3

    B1 --> C1
    B2 --> C2
    B3 --> C3

    style A1 fill:#ffe1e1
    style A2 fill:#ffe1e1
    style A3 fill:#ffe1e1
    style C1 fill:#e1f5e1
    style C2 fill:#e1f5e1
    style C3 fill:#e1f5e1
```

## 10. 完整的请求生命周期

```mermaid
timeline
    title 飞书机器人请求生命周期
    section 用户侧
        发送消息 @机器人 : 0ms
        看到卡片创建 : ~150ms
        看到首次文本 : ~300ms
        看到打字机效果 : 300-3000ms
        收到完整回复 : ~3000ms
    section 飞书平台
        接收消息 : 0ms
        推送 WebSocket 事件 : ~50ms
        创建卡片实体 : ~100ms
        发送卡片到聊天 : ~150ms
        处理卡片更新 : 每 100ms
    section 机器人侧
        接收事件 : ~50ms
        验证与解析 : ~70ms
        启动 Claude CLI : ~100ms
        解析 stream-json : 持续进行
        更新卡片 API : 每 100ms
    section Claude CLI
        启动进程 : ~100ms
        建立会话 : ~150ms
        生成响应 : 持续进行
        输出 stream-json : 流式输出
```
