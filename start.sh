#!/bin/bash
cd web && npm run build && cd ..
nohup go run main.go > logs/mixapi.log 2>&1 &
echo "服务已启动! 端口: 9003"
