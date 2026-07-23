#!/usr/bin/env python3
"""Restart the Cloudflare tunnel with the current Mattermost container IP.

Reads Cloudflare credentials from ~/internkim-poc/cf.env, updates the tunnel
ingress to the current mattermost IP, and starts the cf-tunnel container.

Run this after starting infra containers (mattermost IP may change on restart).
"""
import glob, subprocess, json, re, time, urllib.request, os

CONTAINER = "/opt/homebrew/bin/container"
BASE = os.path.expanduser("~/internkim-poc")
TUNNEL_NAME = "internkim-poc-0"
PUBLIC_HOST = "poc-0.intern.kim"


def container_ip(name):
    out = subprocess.run([CONTAINER, "inspect", name], capture_output=True, text=True)
    return json.loads(out.stdout)[0]["status"]["networks"][0]["ipv4Address"].split("/")[0]


def tenant_flow_routes():
    listing = subprocess.run([CONTAINER, "ls"], capture_output=True, text=True)
    routes = []
    for line in listing.stdout.splitlines():
        fields = line.split()
        if not fields:
            continue
        match = re.match(r"^poc-tenant-(\d+)$", fields[0])
        if not match:
            continue
        routes.append({
            "hostname": f"poc0-t{match.group(1)}.intern.kim",
            "service": f"http://{container_ip(fields[0])}:18080",
        })
    routes.sort(key=lambda route: route["hostname"])
    return routes


def configured_tenant_count():
    pattern = os.path.join(BASE, "config", "tenant_*")
    return len([path for path in glob.glob(pattern)
                if re.match(r"^tenant_[0-9]+$", os.path.basename(path))])


def stable_tenant_flow_routes():
    expected_count = configured_tenant_count()
    deadline = time.monotonic() + 600
    routes = tenant_flow_routes()
    while len(routes) < expected_count and time.monotonic() < deadline:
        time.sleep(5)
        routes = tenant_flow_routes()
    if len(routes) < expected_count:
        print(f"warning: only {len(routes)} of {expected_count} tenant containers are visible")
        print_tenant_diagnostics()
    return routes


def print_tenant_diagnostics():
    listing = subprocess.run([CONTAINER, "ls", "-a"], capture_output=True, text=True)
    tenant_lines = [line for line in listing.stdout.splitlines() if "poc-tenant" in line or "STATE" in line.upper()]
    print("container ls -a (tenants):")
    for line in tenant_lines[:18]:
        print("  " + line)
    logs = subprocess.run([CONTAINER, "logs", "poc-tenant-01"], capture_output=True, text=True)
    print("poc-tenant-01 logs (tail):")
    for line in (logs.stdout or logs.stderr).splitlines()[-15:]:
        print("  " + line)


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

    flow_routes = stable_tenant_flow_routes()
    print(f"flow_routes={len(flow_routes)}")

    ingress = [{"hostname": PUBLIC_HOST, "service": f"http://{mattermost_ip}:8065"}]
    ingress += flow_routes
    ingress.append({"service": "http_status:404"})

    cf("PUT", f"/accounts/{account_id}/cfd_tunnel/{tunnel_id}/configurations",
        {"config": {"ingress": ingress}})
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
