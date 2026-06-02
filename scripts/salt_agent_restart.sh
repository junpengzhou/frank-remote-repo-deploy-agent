#!/bin/bash

# Salt-Agent 容器重启脚本
# 用法: ./salt_agent_restart.sh <模块名称>

set -e

# 检查参数
if [ $# -lt 1 ]; then
    echo "用法: $0 <模块名称>"
    echo ""
    echo "示例:"
    echo "  $0 frank"
    echo "  $0 example-frank"
    echo ""
    exit 1
fi

MODULE_NAME="$1"

echo "========================================="
echo "开始重启 ${MODULE_NAME} Docker 容器..."
echo "========================================="

# 1. 停止容器（忽略错误）
echo "[1/3] 停止容器 ${MODULE_NAME}..."
docker stop "${MODULE_NAME}" 2>/dev/null || {
    echo "警告: 容器 ${MODULE_NAME} 可能未运行或停止失败，继续执行..."
}

# 等待容器完全停止
echo "等待容器完全停止..."
sleep 2

# 2. 启动容器（失败则抛出错误）
echo "[2/3] 启动容器 ${MODULE_NAME}..."
if ! docker start "${MODULE_NAME}"; then
    echo "错误: 容器 ${MODULE_NAME} 启动失败！"
    exit 1
fi
echo "容器 ${MODULE_NAME} 启动成功"

# 3. 验证容器状态
echo "[3/3] 验证容器状态..."
sleep 3
if docker ps | grep -q "${MODULE_NAME}"; then
    echo "容器 ${MODULE_NAME} 运行正常"
else
    echo "警告: 容器 ${MODULE_NAME} 可能未正常运行，请检查日志"
    docker logs --tail 20 "${MODULE_NAME}"
fi

echo ""
echo "========================================="
echo "容器 ${MODULE_NAME} 重启完成！"
echo "========================================="

# 4. 推送更新通知到 Teams
NOTIFY_SCRIPT="/prosh/salt-agent/notify/notify_teams_webhooks.py"
NOTIFY_CONFIG="/prosh/salt-agent/notify/settings.json"

if [ -f "$NOTIFY_SCRIPT" ] && [ -f "$NOTIFY_CONFIG" ]; then
    echo "推送更新通知到 Teams..."
    cd /prosh/salt-agent/ || exit
    python3 "$NOTIFY_SCRIPT" \
        --modules "${MODULE_NAME}" \
        --show-all-containers \
        --config "$NOTIFY_CONFIG"
    echo "通知推送完成"
else
    echo "跳过通知推送（通知脚本或配置文件不存在）"
fi

exit 0