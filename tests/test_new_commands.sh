#!/bin/bash

# 测试新的简化业务命令接口
echo "=== 测试新的简化业务命令接口 ==="

# 启动控制系统
echo "启动控制系统..."
./bin/control &
CONTROL_PID=$!

# 等待控制系统启动
sleep 3

# 启动simulator（使用新的业务命令）
echo "启动simulator..."
./bin/simulator -duration 15s &
SIMULATOR_PID=$!

# 等待测试完成
sleep 18

# 清理进程
echo "清理进程..."
kill $CONTROL_PID $SIMULATOR_PID 2>/dev/null
wait $CONTROL_PID $SIMULATOR_PID 2>/dev/null

echo "测试完成！"
echo ""
echo "=== 预期结果 ==="
echo "1. simulator 应该主要使用新的 business_command 类型"
echo "2. 80% 的时间发送简化命令，20% 的时间发送旧命令"
echo "3. 系统应该能够处理两种类型的命令"
echo "4. 新的命令应该通过智能路由到相应的实现"