#!/bin/bash

# 飞书 CardKit 2.0 API 测试脚本
# 用于验证流式更新功能

# 加载环境变量
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
if [ -f "$PROJECT_ROOT/.env" ]; then
    source "$PROJECT_ROOT/.env"
fi

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 打印函数
print_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

print_error() {
    echo -e "${RED}✗ $1${NC}"
}

print_info() {
    echo -e "${YELLOW}➜ $1${NC}"
}

# 检查环境变量
check_env() {
    print_info "检查环境变量..."

    if [ -z "$FEISHU_APP_ID" ]; then
        print_error "FEISHU_APP_ID 未设置"
        echo "请运行: export FEISHU_APP_ID=cli_xxxxxxxx"
        exit 1
    fi

    if [ -z "$FEISHU_APP_SECRET" ]; then
        print_error "FEISHU_APP_SECRET 未设置"
        echo "请运行: export FEISHU_APP_SECRET=your_secret"
        exit 1
    fi

    if [ -z "$FEISHU_TEST_CHAT_ID" ]; then
        print_error "FEISHU_TEST_CHAT_ID 未设置"
        echo "请运行: export FEISHU_TEST_CHAT_ID=oc_xxxxxxxx"
        exit 1
    fi

    print_success "环境变量检查通过"
    echo "  App ID: $FEISHU_APP_ID"
    echo "  Chat ID: $FEISHU_TEST_CHAT_ID"
    echo
}

# 获取 tenant_access_token
get_token() {
    print_info "获取 tenant_access_token..."

    response=$(curl -s -X POST "https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal" \
        -H "Content-Type: application/json" \
        -d "{
            \"app_id\": \"$FEISHU_APP_ID\",
            \"app_secret\": \"$FEISHU_APP_SECRET\"
        }")

    code=$(echo $response | jq -r '.code')
    if [ "$code" != "0" ]; then
        print_error "获取 token 失败"
        echo $response | jq '.'
        exit 1
    fi

    TOKEN=$(echo $response | jq -r '.tenant_access_token')
    EXPIRE=$(echo $response | jq -r '.expire')

    print_success "Token 获取成功"
    echo "  Token: ${TOKEN:0:20}..."
    echo "  有效期: $EXPIRE 秒"
    echo
}

# 测试创建流式卡片
test_create_card() {
    print_info "测试 1: 创建流式卡片..."

    CARD_CONTENT=$(cat <<EOF
{
  "schema": "2.0",
  "header": {
    "title": {
      "content": "Claude 对话",
      "tag": "plain_text"
    }
  },
  "config": {
    "streaming_mode": true,
    "summary": {
      "content": ""
    },
    "streaming_config": {
      "print_frequency_ms": {
        "default": 70,
        "android": 70,
        "ios": 70,
        "pc": 70
      },
      "print_step": {
        "default": 1,
        "android": 1,
        "ios": 1,
        "pc": 1
      },
      "print_strategy": "fast"
    }
  },
  "body": {
    "elements": [
      {
        "tag": "markdown",
        "content": "思考中...",
        "element_id": "content_markdown"
      }
    ]
  }
}
EOF
)

    create_payload=$(jq -n --arg card "$CARD_CONTENT" '{type:"card_json", data:$card}')

    response=$(curl -s -X POST "https://open.feishu.cn/open-apis/cardkit/v1/cards" \
        -H "Authorization: Bearer $TOKEN" \
        -H "Content-Type: application/json" \
        -d "$create_payload")

    code=$(echo $response | jq -r '.code')
    if [ "$code" != "0" ]; then
        print_error "创建卡片实体失败"
        echo $response | jq '.'
        exit 1
    fi

    CARD_ID=$(echo $response | jq -r '.data.card_id')
    ELEMENT_ID="content_markdown"

    send_payload=$(jq -n --arg card_id "$CARD_ID" --arg chat_id "$FEISHU_TEST_CHAT_ID" '{
      receive_id: $chat_id,
      msg_type: "interactive",
      content: ("{\"type\":\"card\",\"data\":{\"card_id\":\"" + $card_id + "\"}}")
    }')

    send_resp=$(curl -s -X POST "https://open.feishu.cn/open-apis/im/v1/messages?receive_id_type=chat_id" \
        -H "Authorization: Bearer $TOKEN" \
        -H "Content-Type: application/json" \
        -d "$send_payload")

    send_code=$(echo $send_resp | jq -r '.code')
    if [ "$send_code" != "0" ]; then
        print_error "发送卡片失败"
        echo $send_resp | jq '.'
        exit 1
    fi

    print_success "卡片创建并发送成功"
    echo "  Card ID: $CARD_ID"
    echo "  Element ID: $ELEMENT_ID"
    echo

    # 保存到文件供后续使用
    echo "CARD_ID=$CARD_ID" > /tmp/feishu_test_card.env
    echo "ELEMENT_ID=$ELEMENT_ID" >> /tmp/feishu_test_card.env
    echo "TOKEN=$TOKEN" >> /tmp/feishu_test_card.env
}

# 测试流式更新
test_stream_update() {
    print_info "测试 2: 流式更新卡片内容..."

    if [ ! -f /tmp/feishu_test_card.env ]; then
        print_error "未找到卡片信息，请先运行创建卡片测试"
        exit 1
    fi

    source /tmp/feishu_test_card.env

    # 模拟流式输出
    messages=("Hello" "Hello there" "Hello there!" "Hello there! This" "Hello there! This is" "Hello there! This is a" "Hello there! This is a streaming" "Hello there! This is a streaming test" "Hello there! This is a streaming test.")

    for i in "${!messages[@]}"; do
        sequence=$((i + 1))
        content="${messages[$i]}"

        print_info "更新 $sequence: \"$content\""

        response=$(curl -s -X PUT "https://open.feishu.cn/open-apis/cardkit/v1/cards/$CARD_ID/elements/$ELEMENT_ID/content" \
            -H "Authorization: Bearer $TOKEN" \
            -H "Content-Type: application/json" \
            -d "{
                \"content\": \"$content\",
                \"sequence\": $sequence
            }")

        code=$(echo $response | jq -r '.code')
        if [ "$code" != "0" ]; then
            print_error "更新失败 (sequence=$sequence)"
            echo $response | jq '.'
            exit 1
        fi

        print_success "更新成功"
        sleep 0.5
    done

    echo
    print_success "所有更新完成！"
    echo
}

# 主函数
main() {
    echo "=========================================="
    echo "  飞书 CardKit 2.0 流式更新 API 测试"
    echo "=========================================="
    echo

    # 检查依赖
    if ! command -v jq &> /dev/null; then
        print_error "未安装 jq，请运行: brew install jq"
        exit 1
    fi

    # 检查环境变量
    check_env

    # 获取 token
    get_token

    # 创建卡片
    test_create_card

    # 测试流式更新
    test_stream_update

    echo "=========================================="
    print_success "所有测试通过！"
    echo "=========================================="
    echo
    echo "请在飞书群聊中查看卡片，应该能看到打字机效果。"
    echo
    echo "如果效果正常，说明 CardKit 2.0 API 可用，可以继续开发完整系统！"
}

# 运行主函数
main
