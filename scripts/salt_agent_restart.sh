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

# 1. 停止容器（忽略错误）,等待优雅下线时间为90秒
echo "[1/2] 停止容器 ${MODULE_NAME}..."
docker stop "${MODULE_NAME}" -t 90 2>/dev/null || {
    echo "警告: 容器 ${MODULE_NAME} 可能未运行或停止失败，继续执行..."
}

# 2. 启动容器（失败则抛出错误），增加防控attaching to network failed的异常，提升稳定性
echo "[2/2] 启动容器 ${MODULE_NAME}..."
START_OUTPUT=$(docker start "${MODULE_NAME}" 2>&1)
START_EXIT_CODE=$?

if [ $START_EXIT_CODE -ne 0 ]; then
    # 检查是否包含网络错误信息
    if echo "$START_OUTPUT" | grep -q "attaching to network failed"; then
        echo "检测到网络附加错误，尝试重新连接网络..."
        sleep 3
        
        # 从容器配置中获取网络名称
        NETWORK_NAME=$(docker inspect -f '{{range $key, $value := .NetworkSettings.Networks}}{{$key}}{{end}}' "${MODULE_NAME}" 2>/dev/null | head -n 1)
        
        if [ -z "$NETWORK_NAME" ]; then
            echo "错误: 无法获取容器 ${MODULE_NAME} 的网络名称！"
            echo "原始错误: $START_OUTPUT"
            exit 1
        fi
        
        echo "检测到网络名称: ${NETWORK_NAME}"
        
        # 先断开网络连接（如果已连接）
        docker network disconnect "${NETWORK_NAME}" "${MODULE_NAME}" 2>/dev/null || true
        sleep 1
        
        # 重新连接到正确的网络
        echo "执行: docker network connect ${NETWORK_NAME} ${MODULE_NAME}"
        if docker network connect "${NETWORK_NAME}" "${MODULE_NAME}" 2>/dev/null; then
            echo "网络连接成功，重新启动容器..."
            sleep 1
            
            # 重新启动容器
            if ! docker start "${MODULE_NAME}"; then
                echo "错误: 容器 ${MODULE_NAME} 重新启动失败！"
                exit 1
            fi
            echo "容器 ${MODULE_NAME} 启动成功"
        else
            echo "错误: 无法连接到网络 ${NETWORK_NAME}，容器启动失败！"
            echo "原始错误: $START_OUTPUT"
            exit 1
        fi
    else
        echo "错误: 容器 ${MODULE_NAME} 启动失败！"
        echo "错误信息: $START_OUTPUT"
        exit 1
    fi
else
    echo "容器 ${MODULE_NAME} 启动成功"
fi

echo ""
echo "========================================="
echo "容器 ${MODULE_NAME} 重启完成！"
echo "========================================="

exit 0