#!/bin/bash

# 1. 定义Salt-Agent路径
SALT_AGENT_HOME="/data/salt-agent"

# 2. 检查salt-agent是否已经可以直接执行
if command -v salt-agent &> /dev/null; then
    echo "salt-agent is already installed and available."
    salt-agent --version || salt-agent -v || echo "salt-agent version unknown"
    exit 0
fi

# 3. 一键配置环境变量
echo "export SALT_AGENT_HOME=${SALT_AGENT_HOME}" | sudo tee /etc/profile.d/salt-agent.sh
echo "export PATH=\$PATH:\$SALT_AGENT_HOME" | sudo tee -a /etc/profile.d/salt-agent.sh

# 4. 提供用户指引
echo ""
echo "========================================="
echo "Salt-Agent environment configured successfully!"
echo "========================================="
echo ""
echo "Environment variables have been configured in:"
echo "  /etc/profile.d/salt-agent.sh"
echo ""
echo "To use Salt-Agent immediately, please run:"
echo "  source /etc/profile.d/salt-agent.sh"
echo ""
echo "Or simply log out and log back in."
echo "========================================="
