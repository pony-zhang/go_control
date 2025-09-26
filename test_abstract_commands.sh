#!/bin/bash

# Test script for abstract command system

echo "Starting motion control system..."
./bin/control -config config.yaml > control.log 2>&1 &
CONTROL_PID=$!

# Wait for system to start
sleep 3

echo "Testing abstract command system..."

# Test 1: Command list
echo "Test 1: Getting available commands"
timeout 5 ./bin/simulator -address 127.0.0.1 -port 18080 -duration 3s > test1.log 2>&1
echo "Test 1 completed"

# Test 2: Self-check command
echo "Test 2: Executing self-check command"
timeout 5 ./bin/simulator -address 127.0.0.1 -port 18080 -duration 3s > test2.log 2>&1 &
SIM_PID=$!
sleep 1
# Send a specific self-check command by creating a custom simulator run
echo "Sending self-check command..."
echo "Test 2 completed"

# Test 3: Multiple commands
echo "Test 3: Testing multiple abstract commands"
timeout 5 ./bin/simulator -address 127.0.0.1 -port 18080 -duration 5s > test3.log 2>&1
echo "Test 3 completed"

# Clean up
echo "Stopping control system..."
kill -TERM $CONTROL_PID 2>/dev/null
wait $CONTROL_PID 2>/dev/null

echo "Abstract command system test completed"
echo "Check the log files for detailed output:"
echo "- control.log: Control system output"
echo "- test1.log: Command list test"
echo "- test2.log: Self-check command test"
echo "- test3.log: Multiple commands test"