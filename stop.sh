#!/bin/bash

# 查找包含"main.go"的进程并获取PID
PID=$(ps aux | grep "main.go" | grep -v grep | awk '{print $2}')

# 检查是否找到了进程
if [ -z "$PID" ]; then
    echo "没有找到运行中的 main.go 进程"
else
    # 逐个终止找到的进程
    for p in $PID; do
        echo "正在终止进程: $p"
        kill -9 $p
    done
    echo "所有 main.go 进程已终止"
fi


# 检查端口 9003 是否有进程占用 退出容器到服务器执行
PID=$(ss -tlnp 2>/dev/null | grep ':9003' | grep -oP 'pid=\K[0-9]+')

if [ -n "$PID" ]; then
    echo "端口 9003 被占用 PID: $PID"
    echo "正在终止进程..."
    kill -9 $PID
    sleep 1

    # 验证是否已释放
    if ss -tlnp 2>/dev/null | grep -q ':9003'; then
        echo "警告：端口仍被占用"
    else
        echo "端口 9003 已释放"
    fi
else
    echo "端口 9003 空闲"
fi
