#!/bin/bash

# SSH 密钥自动配置脚本
# 用法: ./install_remote_server.sh <服务器IP> <端口> <用户名> <密码>

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 检查参数
if [ $# -lt 4 ]; then
    echo -e "${RED}用法: $0 <服务器IP> <端口> <用户名> <密码>${NC}"
    echo ""
    echo "示例:"
    echo "  $0 192.168.60.233 22 root your_password"
    echo ""
    exit 1
fi

REMOTE_HOST="$1"
REMOTE_PORT="$2"
REMOTE_USER="$3"
REMOTE_PASS="$4"

SSH_DIR="/data/salt-agent/.ssh/${REMOTE_HOST}"
PRIVATE_KEY="${SSH_DIR}/id_rsa"
PUBLIC_KEY="${SSH_DIR}/id_rsa.pub"

echo -e "${GREEN}=========================================${NC}"
echo -e "${GREEN}SSH 密钥自动配置脚本${NC}"
echo -e "${GREEN}=========================================${NC}"
echo ""

# 1. 检查 sshpass 是否安装
if ! command -v sshpass &> /dev/null; then
    echo -e "${YELLOW}检测到 sshpass 未安装，正在安装...${NC}"
    if command -v yum &> /dev/null; then
        sudo yum install -y sshpass
    elif command -v apt-get &> /dev/null; then
        sudo apt-get install -y sshpass
    else
        echo -e "${RED}错误: 无法自动安装 sshpass，请手动安装后重试${NC}"
        exit 1
    fi
    echo -e "${GREEN}sshpass 安装成功${NC}"
fi

# 2. 创建 .ssh 目录
echo -e "${YELLOW}[1/5] 创建 SSH 目录...${NC}"
mkdir -p "$SSH_DIR"
chmod 700 "$SSH_DIR"

# 3. 生成 SSH 密钥对（如果不存在）
if [ -f "$PRIVATE_KEY" ] && [ -f "$PUBLIC_KEY" ]; then
    echo -e "${YELLOW}SSH 密钥已存在，是否重新生成？(y/n)${NC}"
    read -r regenerate
    if [ "$regenerate" != "y" ] && [ "$regenerate" != "Y" ]; then
        echo -e "${GREEN}使用现有密钥${NC}"
    else
        echo -e "${YELLOW}正在生成新的 SSH 密钥对...${NC}"
        ssh-keygen -t rsa -b 4096 -f "$PRIVATE_KEY" -N "" -q
        echo -e "${GREEN}SSH 密钥对生成成功${NC}"
    fi
else
    echo -e "${YELLOW}正在生成 SSH 密钥对...${NC}"
    ssh-keygen -t rsa -b 4096 -f "$PRIVATE_KEY" -N "" -q
    chmod 600 "$PRIVATE_KEY"
    chmod 644 "$PUBLIC_KEY"
    echo -e "${GREEN}SSH 密钥对生成成功${NC}"
fi

# 4. 上传公钥到远程服务器
echo -e "${YELLOW}[2/5] 上传公钥到远程服务器 ${REMOTE_USER}@${REMOTE_HOST}:${REMOTE_PORT}...${NC}"

# 使用 sshpass 自动输入密码上传公钥
sshpass -p "$REMOTE_PASS" ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -p "$REMOTE_PORT" \
    "${REMOTE_USER}@${REMOTE_HOST}" "
    mkdir -p ~/.ssh && \
    chmod 700 ~/.ssh && \
    cat >> ~/.ssh/authorized_keys && \
    chmod 600 ~/.ssh/authorized_keys && \
    echo 'Public key added successfully'
" < "$PUBLIC_KEY"

if [ $? -eq 0 ]; then
    echo -e "${GREEN}公钥上传成功${NC}"
else
    echo -e "${RED}公钥上传失败${NC}"
    exit 1
fi

# 5. 验证 SSH 连接
echo -e "${YELLOW}[3/5] 验证 SSH 连接...${NC}"
sshpass -p "$REMOTE_PASS" ssh -o StrictHostKeyChecking=no -p "$REMOTE_PORT" \
    "${REMOTE_USER}@${REMOTE_HOST}" "echo 'SSH connection successful'" > /dev/null 2>&1

if [ $? -eq 0 ]; then
    echo -e "${GREEN}SSH 连接验证成功${NC}"
else
    echo -e "${RED}SSH 连接验证失败${NC}"
    exit 1
fi

# 6. 测试免密登录
echo -e "${YELLOW}[4/5] 测试免密登录...${NC}"
ssh -i "$PRIVATE_KEY" -o StrictHostKeyChecking=no -p "$REMOTE_PORT" \
    "${REMOTE_USER}@${REMOTE_HOST}" "echo 'Passwordless login successful'" > /dev/null 2>&1

if [ $? -eq 0 ]; then
    echo -e "${GREEN}免密登录配置成功${NC}"
else
    echo -e "${RED}免密登录配置失败，请检查配置${NC}"
    exit 1
fi

# 7. 显示配置信息
echo -e "${YELLOW}[5/5] 生成配置文件信息...${NC}"
echo ""
echo -e "${GREEN}=========================================${NC}"
echo -e "${GREEN}SSH 配置完成！${NC}"
echo -e "${GREEN}=========================================${NC}"
echo ""
echo -e "私钥位置: ${YELLOW}${PRIVATE_KEY}${NC}"
echo -e "公钥位置: ${YELLOW}${PUBLIC_KEY}${NC}"
echo ""
echo -e "远程服务器信息:"
echo -e "  Host: ${YELLOW}${REMOTE_HOST}${NC}"
echo -e "  Port: ${YELLOW}${REMOTE_PORT}${NC}"
echo -e "  User: ${YELLOW}${REMOTE_USER}${NC}"
echo ""
echo -e "在 agent.yaml 中配置 SSH 部分:"
echo -e "${YELLOW}ssh:${NC}"
echo -e "  user: ${YELLOW}${REMOTE_USER}${NC}"
echo -e "  host: ${YELLOW}${REMOTE_HOST}${NC}"
echo -e "  port: ${YELLOW}${REMOTE_PORT}${NC}"
echo -e "  keyFile: ${YELLOW}${PRIVATE_KEY}${NC}"
echo ""
echo -e "${GREEN}现在可以使用免密方式连接到远程服务器了！${NC}"
echo -e "${GREEN}=========================================${NC}"

exit 0
