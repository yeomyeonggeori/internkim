#!/bin/bash
# Usage: ./set-api-key.sh sk-or-v1-xxxxx

if [ -z "$1" ]; then
  echo "Usage: $0 <OPENROUTER_API_KEY>"
  exit 1
fi

KEY="$1"

sshpass -p root ssh -o StrictHostKeyChecking=no root@192.168.0.141 "python3 << 'PYEOF'
import json
with open('/root/.picoclaw/config.json') as f:
    cfg = json.load(f)
cfg['model_list'][0]['api_key'] = '$KEY'
with open('/root/.picoclaw/config.json', 'w') as f:
    json.dump(cfg, f, indent=2)
print('API key set')
PYEOF
killall picoclaw 2>/dev/null; sleep 1
nohup /usr/local/bin/picoclaw gateway > /var/log/picoclaw.log 2>&1 &
sleep 2
echo 'picoclaw restarted'
"
