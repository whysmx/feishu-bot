#!/bin/bash

# Test script for webhook service
echo "Testing Feishu Bot Webhook Service"

# Load environment variables
source .env

# Start webhook service in background (if not already running)
if ! pgrep -f "./webhook" > /dev/null; then
    echo "Starting webhook service..."
    ./webhook &
    WEBHOOK_PID=$!
    sleep 2
else
    echo "Webhook service already running"
fi

# Wait for service to be ready
echo "Waiting for webhook service to be ready..."
while ! curl -s http://localhost:8080/health > /dev/null; do
    sleep 1
done

echo "Webhook service is ready!"

# Test notification endpoint
echo "Testing notification endpoint..."
curl -X POST http://localhost:8080/webhook/notification \
  -H "Content-Type: application/json" \
  -d '{
    "type": "completed",
    "user_id": "ou_8a4ad14f0daec82d332888e5ee31ad82", 
    "open_id": "ou_8a4ad14f0daec82d332888e5ee31ad82",
    "project_name": "test-project",
    "description": "Task completed successfully",
    "working_dir": "/Users/test/project",
    "tmux_session": "claude-code"
  }'

echo ""
echo "Test completed! Check the webhook logs for details."

# If we started the service, clean up
if [ ! -z "$WEBHOOK_PID" ]; then
    echo "Stopping webhook service..."
    kill $WEBHOOK_PID
fi