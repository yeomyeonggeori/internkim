#!/usr/bin/env python3
"""Set up a persistent Cloudflare SSH tunnel on the macOS host.

Creates tunnel internkim-poc-macstudio-ssh pointing to ssh://localhost:22,
writes a LaunchAgent plist, and loads it. Run once on macstudio.

Prerequisites:
  brew install cloudflare/cloudflare/cloudflared
  ~/internkim-poc/cf.env must contain CF_ACCOUNT_ID, CF_ZONE_ID, and
  one of: CLOUDFLARE_API_TOKEN, CF_API_TOKEN, CLOUDFLARE_TOKEN

After setup, connect from any machine:
  ssh -o ProxyCommand='cloudflared access ssh --hostname %h' dawn@ssh-poc.intern.kim
"""
import subprocess, json, re, urllib.request, os, plistlib

BASE = os.path.expanduser("~/internkim-poc")
TUNNEL_NAME = "internkim-poc-macstudio-ssh"
PUBLIC_HOST = "ssh-poc.intern.kim"
PLIST_LABEL = "kim.intern.poc.ssh-tunnel"
PLIST_PATH = os.path.expanduser(f"~/Library/LaunchAgents/{PLIST_LABEL}.plist")
CLOUDFLARED = "/opt/homebrew/bin/cloudflared"


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
    zone_id = env["CF_ZONE_ID"]

    cf = lambda method, path, data=None: cloudflare_request(method, path, token, data)

    tunnels = cf("GET", f"/accounts/{account_id}/cfd_tunnel?name={TUNNEL_NAME}&is_deleted=false")
    existing = tunnels["result"]
    if existing:
        tunnel_id = existing[0]["id"]
        print(f"tunnel=reused:{tunnel_id}")
    else:
        created = cf("POST", f"/accounts/{account_id}/cfd_tunnel",
            {"name": TUNNEL_NAME, "config_src": "cloudflare"})
        tunnel_id = created["result"]["id"]
        print(f"tunnel=created:{tunnel_id}")

    cf("PUT", f"/accounts/{account_id}/cfd_tunnel/{tunnel_id}/configurations",
        {"config": {"ingress": [
            {"hostname": PUBLIC_HOST, "service": "ssh://localhost:22"},
            {"service": "http_status:404"}
        ]}})
    print("ingress=updated")

    cname_target = f"{tunnel_id}.cfargotunnel.com"
    records = cf("GET", f"/zones/{zone_id}/dns_records?type=CNAME&name={PUBLIC_HOST}")
    payload = {"type": "CNAME", "name": PUBLIC_HOST, "content": cname_target, "proxied": True}
    if records["result"]:
        cf("PUT", f"/zones/{zone_id}/dns_records/{records['result'][0]['id']}", payload)
    else:
        cf("POST", f"/zones/{zone_id}/dns_records", payload)
    print(f"dns=ok:{PUBLIC_HOST}")

    connector_token = cf("GET", f"/accounts/{account_id}/cfd_tunnel/{tunnel_id}/token")["result"]

    plist = {
        "Label": PLIST_LABEL,
        "ProgramArguments": [
            CLOUDFLARED, "tunnel", "--no-autoupdate", "run", "--token", connector_token
        ],
        "RunAtLoad": True,
        "KeepAlive": True,
        "StandardOutPath": os.path.join(BASE, "ssh-tunnel.log"),
        "StandardErrorPath": os.path.join(BASE, "ssh-tunnel.log"),
        "EnvironmentVariables": {"PATH": "/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin"},
    }
    with open(PLIST_PATH, "wb") as f:
        plistlib.dump(plist, f)
    print(f"plist=written:{PLIST_PATH}")

    subprocess.run(["launchctl", "unload", PLIST_PATH], capture_output=True)
    result = subprocess.run(["launchctl", "load", PLIST_PATH], capture_output=True, text=True)
    print("launchctl=" + ("loaded" if result.returncode == 0 else "FAILED: " + result.stderr))

    print(f"\nSSH from any network:")
    print(f"  ssh -o ProxyCommand='cloudflared access ssh --hostname %h' dawn@{PUBLIC_HOST}")
    print(f"\nAdd to ~/.ssh/config on client:")
    print(f"  Host {PUBLIC_HOST}")
    print(f"    ProxyCommand cloudflared access ssh --hostname %h")


if __name__ == "__main__":
    main()
