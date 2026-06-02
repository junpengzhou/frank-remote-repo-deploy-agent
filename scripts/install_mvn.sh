#!/bin/bash

# 1. 定义要安装的 Maven 版本
MAVEN_HOME="/data/salt-agent/maven"

TAR_DIR="/data/salt-agent/apache-maven-3.9.16-bin.tar.gz"

# 检查并配置 JAVA_HOME
JAVA_HOME="/data/salt-agent/java/jdk21"

if [ -d "$JAVA_HOME" ] && [ -x "$JAVA_HOME/bin/java" ]; then
    echo "Found Java installation at $JAVA_HOME, configuring JAVA_HOME..."
    export JAVA_HOME="${JAVA_HOME}"
    echo "export JAVA_HOME=${JAVA_HOME}" | sudo tee /etc/profile.d/java.sh
    echo "export PATH=\$PATH:\$JAVA_HOME/bin" | sudo tee -a /etc/profile.d/java.sh
    source /etc/profile.d/java.sh
    echo "JAVA_HOME configured successfully."
else
    echo "Error: JAVA_HOME not set or Java not found at $JAVA_HOME"
    exit 1
fi

# 判断如果INSTALL_DIR存在且其中的mvn可执行文件存在，则跳过解压
if [ -d "$MAVEN_HOME" ] && [ -x "$MAVEN_HOME/bin/mvn" ]; then
    echo "Maven already installed at $MAVEN_HOME, skipping extraction."
else
    if [ ! -f "$TAR_DIR" ]; then
        echo "Error: Maven tar file not found at $TAR_DIR"
        exit 1
    fi

    echo "Extracting Maven to $MAVEN_HOME..."
    mkdir -p "$MAVEN_HOME"
    if ! tar -zxvf "$TAR_DIR" -C "$(dirname $MAVEN_HOME)" --strip-components=1; then
        echo "Error: Failed to extract Maven"
        exit 1
    fi
    echo "Maven extracted successfully."
fi

# 2. 一键配置环境变量
echo "export MAVEN_HOME=${MAVEN_HOME}" | tee /etc/profile.d/maven.sh
echo "export PATH=\$PATH:\$MAVEN_HOME/bin" | sudo tee -a /etc/profile.d/maven.sh

# 3. 使环境变量立即生效
source /etc/profile.d/maven.sh

# 4. 验证安装
mvn -v