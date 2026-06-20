#!/bin/bash
# Called by LaunchAgent at login. Waits for Apple Container runtime, then starts infra and tenants.
export PATH=/opt/homebrew/bin:$PATH

until container system status 2>/dev/null | grep -q 'running'; do sleep 2; done
container start poc-postgres 2>/dev/null || true
sleep 5
container start poc-mattermost 2>/dev/null || true
sleep 10
python3 "$(dirname "$0")/start-poc.py"
python3 "$(dirname "$0")/restart-tunnel.py"
