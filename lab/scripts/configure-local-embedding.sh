#!/usr/bin/env bash
set -euo pipefail

cacheDirectory="${1:-/mnt/shared/workspace/.dependency/local-fleet-embedding}"
runtimeDirectory="$cacheDirectory/runtime/llama-b9660"
modelPath="$cacheDirectory/bge-m3-Q8_0.gguf"

test -x "$runtimeDirectory/llama-server"
test -s "$modelPath"
if ! ldconfig -p | grep -q 'libgomp.so.1'; then
  apt-get update >/dev/null
  DEBIAN_FRONTEND=noninteractive apt-get install -y libgomp1 >/dev/null
fi
install -d -m 0755 /usr/local/lib/llama-cpp /root/.internkim/models
systemctl stop internkim-llamacpp-embedding.service 2>/dev/null || true
systemctl reset-failed internkim-llamacpp-embedding.service 2>/dev/null || true
cp -a "$runtimeDirectory"/. /usr/local/lib/llama-cpp/
chmod 0755 /usr/local/lib/llama-cpp/llama-server
ln -sf /usr/local/lib/llama-cpp/llama-server /usr/local/bin/llama-server
install -m 0600 "$modelPath" /root/.internkim/models/bge-m3-Q8_0.gguf

cat >/etc/systemd/system/internkim-llamacpp-embedding.service <<'SERVICE'
[Unit]
Description=InternKim llama.cpp Embedding Server
After=network-online.target
Wants=network-online.target

[Service]
User=root
Environment=LD_LIBRARY_PATH=/usr/local/lib/llama-cpp
ExecStart=/usr/local/lib/llama-cpp/llama-server -m /root/.internkim/models/bge-m3-Q8_0.gguf --host 127.0.0.1 --port 18082 -ngl 0 --embeddings --pooling cls --batch-size 2048 --ubatch-size 2048
Restart=on-failure
RestartSec=2
TimeoutStartSec=120

[Install]
WantedBy=multi-user.target
SERVICE

systemctl daemon-reload
systemctl enable internkim-llamacpp-embedding.service
systemctl restart internkim-llamacpp-embedding.service
for attempt in $(seq 1 60); do
  if curl -fs http://127.0.0.1:18082/health >/dev/null 2>&1; then
    break
  fi
  if systemctl is-failed --quiet internkim-llamacpp-embedding.service; then
    journalctl -u internkim-llamacpp-embedding.service -n 40 --no-pager
    exit 1
  fi
  sleep 1
done
curl -fsS http://127.0.0.1:18082/health >/dev/null
response="$(curl -fsS http://127.0.0.1:18082/v1/embeddings -H 'Content-Type: application/json' -d '{"model":"baai/bge-m3","input":"로컬 임베딩 확인"}')"
dimension="$(printf '%s' "$response" | jq -r '.data[0].embedding | length')"
test "$dimension" = "1024"
echo "local embedding ready: baai/bge-m3"
