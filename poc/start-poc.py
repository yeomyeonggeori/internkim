#!/usr/bin/env python3
"""Start/restart PoC tenant containers via Apple Container.

Usage:
  python3 start-poc.py              # start tenants 1-TENANT_COUNT (default 10)
  python3 start-poc.py 15           # start tenants 1-15 (add new tenants)

Infra containers (poc-postgres, poc-mattermost) must already be running.
Patches tenant runtime.json files with current postgres IP before starting.
"""
import os, subprocess, json, sys, glob, re

CONTAINER = '/opt/homebrew/bin/container'
BASE = os.path.expanduser('~/internkim-poc')
NETWORK = 'internkim-poc'
TENANT_IMAGE = 'internkim-poc-tenant:flow'
DEFAULT_TENANT_COUNT = int(os.environ.get('TENANT_COUNT', '10'))


def run(cmd, check=True):
    result = subprocess.run(cmd, capture_output=True, text=True)
    if check and result.returncode != 0:
        print('FAIL:', cmd, file=sys.stderr)
        print(result.stderr, file=sys.stderr)
    return result.stdout.strip()


def container_ip(name):
    out = json.loads(run([CONTAINER, 'inspect', name]))
    return out[0]['status']['networks'][0]['ipv4Address'].split('/')[0]


def patch_ips_in_configs(pg_ip, mm_ip):
    pattern = os.path.join(BASE, 'config', 'tenant_*', 'runtime.json')
    for config_path in sorted(glob.glob(pattern)):
        content = open(config_path).read()
        patched = re.sub(r'@[^@/:]+:5432/', f'@{pg_ip}:5432/', content)
        patched = re.sub(r'http://[^/"]+:8065', f'http://{mm_ip}:8065', patched)
        if patched != content:
            open(config_path, 'w').write(patched)
            tenant = os.path.basename(os.path.dirname(config_path))
            print(f'Patched IPs in {tenant}/runtime.json')


def start_tenant(n, pg_ip, mm_ip):
    name = f'poc-tenant-{n:02d}'
    tenant_id = f'tenant_{n:02d}'
    team_name = f'tenant{n:02d}'
    secrets = os.path.join(BASE, 'secrets', tenant_id)
    config = os.path.join(BASE, 'config', tenant_id)
    workspace = os.path.join(BASE, 'workspace', tenant_id)
    openrouter = os.path.join(BASE, 'secrets', 'openrouter-key')
    os.makedirs(workspace, exist_ok=True)

    existing = run([CONTAINER, 'inspect', name], check=False)
    if existing:
        run([CONTAINER, 'stop', name], check=False)
        run([CONTAINER, 'rm', name], check=False)

    cmd = [CONTAINER, 'run', '--detach', '--name', name, '--network', NETWORK,
        '-e', f'POSTGRES_HOST={pg_ip}', '-e', f'MATTERMOST_HOST={mm_ip}',
        '-e', f'MATTERMOST_TEAM={team_name}', '-e', f'BOT_USERNAME=internkim{n:02d}', '-e', 'ENABLE_ADMIND=1',
        '-v', f'{config}:/etc/blueclaw:rw',
        '-v', f'{workspace}:/workspace:rw',
        '-v', f'{openrouter}:/secrets/openrouter-key:ro',
        '-v', f'{secrets}/mattermost-bot-token:/secrets/mattermost-bot-token:ro',
        '-v', f'{secrets}/mattermost-bot-token:/root/.internkim/secrets/mattermost-bot-token:ro',
        '-v', f'{secrets}/mm-admin-pass:/root/.internkim/secrets/mm-admin-pass:ro',
        '-v', f'{secrets}/admin-email:/root/.internkim/secrets/admin-email:ro',
        '-v', f'{secrets}/device-url:/root/.internkim/env/device-url:ro',
        '-v', f'{secrets}/flow-public-url:/root/.internkim/env/flow-public-url:ro',
        TENANT_IMAGE]
    print(f'Starting {name}...')
    run(cmd)


if __name__ == '__main__':
    count = int(sys.argv[1]) if len(sys.argv) > 1 else DEFAULT_TENANT_COUNT
    pg_ip = container_ip('poc-postgres')
    mm_ip = container_ip('poc-mattermost')
    print(f'Postgres: {pg_ip}, Mattermost: {mm_ip}')
    patch_ips_in_configs(pg_ip, mm_ip)
    for n in range(1, count + 1):
        start_tenant(n, pg_ip, mm_ip)
    print(f'All {count} tenants started')
