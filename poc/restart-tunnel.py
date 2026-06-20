#!/usr/bin/env python3
"""Restart the Cloudflare tunnel with the current Mattermost container IP.

Reads Cloudflare credentials from ~/internkim-poc/cf.env, updates the tunnel
ingress to the current mattermost IP, and starts the cf-tunnel container.

Run this after starting infra containers (mattermost IP may change on restart).
"""
import subprocess, json, re, urllib.request, os

CONTAINER = "/opt/homebrew/bin/container"
BASE = os.path.expanduser("~/internkim-poc")
TUNNEL_NAME = "internkim-poc-0"
PUBLIC_HOST = "poc-0.intern.kim"


def container_ip(name):
    out = subprocess.run([CONTAINER, "inspect", name], capture_output=True, text=True)
    return json.loads(out.stdout)[0]["status"]["networks"][0]["ipv4Address"].split("/")[0]


def read_env(path):
    env = {}
    for line in open(path):
        match = re.match(r"^([A-Z0-9_]+)=(.+)", line.strip())
        if match:
            env[match.group(1)] = match.group(2).strip("\"'\t ")
    return env


def cloudflare_request(method, path, token, data=None):
    url = "https://api.cloudflare.com/client/v4" + path
    body = json.dumps(data).encode() if data else None
    request = urllib.request.Request(url, data=body, method=method,
        headers={"Authorization": "Bearer " + token, "Content-Type": "application/json"})
    with urllib.request.urlopen(request) as response:
        return json.load(response)


def main():
    env = read_env(os.path.join(BASE, "cf" + ".env"))
    token = next((env[k] for k in ["CLOUDFLARE_API_TOKEN", "CF_API_TOKEN", "CLOUDFLARE_TOKEN"] if k in env), None)
    account_id = env["CF_ACCOUNT_ID"]

    cf = lambda method, path, data=None: cloudflare_request(method, path, token, data)

    tunnels = cf("GET", f"/accounts/{account_id}/cfd_tunnel?name={TUNNEL_NAME}&is_deleted=false")
    tunnel_id = tunnels["result"][0]["id"]
    print(f"tunnel_id={tunnel_id}")

    mattermost_ip = container_ip("poc-mattermost")
    print(f"mattermost_ip={mattermost_ip}")

    cf("PUT", f"/accounts/{account_id}/cfd_tunnel/{tunnel_id}/configurations",
        {"config": {"ingress": [
            {"hostname": PUBLIC_HOST, "service": f"http://{mattermost_ip}:8065"},
            {"service": "http_status:404"}
        ]}})
    print("ingress=updated")

    connector_token = cf("GET", f"/accounts/{account_id}/cfd_tunnel/{tunnel_id}/token")["result"]

    subprocess.run([CONTAINER, "stop", "cf-tunnel"], capture_output=True)
    subprocess.run([CONTAINER, "rm", "cf-tunnel"], capture_output=True)
    result = subprocess.run([
        CONTAINER, "run", "--detach", "--name", "cf-tunnel",
        "--network", "internkim-poc",
        "cloudflare/cloudflared:latest",
        "tunnel", "--no-autoupdate", "run", "--token", connector_token
    ], capture_output=True, text=True)
    print("cloudflared=" + ("started" if result.returncode == 0 else "FAILED: " + result.stderr))


if __name__ == "__main__":
    main()
