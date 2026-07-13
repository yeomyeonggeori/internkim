#!/usr/bin/env python3
"""Start/restart PoC tenant containers via Apple Container.

Usage:
  python3 start-poc.py              # start tenants 1-TENANT_COUNT or current config count
  python3 start-poc.py 15           # start tenants 1-15 (add new tenants)

Infra containers (poc-postgres, poc-mattermost) must already be running.
Patches tenant runtime.json files with current postgres IP before starting.
"""
import os, subprocess, json, sys, glob, re

CONTAINER = '/opt/homebrew/bin/container'
BASE = os.path.expanduser('~/internkim-poc')
NETWORK = 'internkim-poc'
TENANT_IMAGE = os.environ.get('TENANT_IMAGE', 'internkim-poc-tenant:flow')
FALLBACK_TENANT_COUNT = 10
DEFAULT_WORKSPACE_SETTINGS = {'timeZone': 'Asia/Seoul', 'language': 'ko'}


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


def initialize_workspace_settings(workspace):
    state_directory = os.path.join(workspace, '.admind', 'state')
    settings_path = os.path.join(state_directory, 'workspace-settings.json')
    os.makedirs(state_directory, exist_ok=True)
    try:
        descriptor = os.open(settings_path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    except FileExistsError:
        return
    with os.fdopen(descriptor, 'w') as settings_file:
        json.dump(DEFAULT_WORKSPACE_SETTINGS, settings_file, ensure_ascii=False, indent=2)
        settings_file.write('\n')


def migrate_flow_database(name, workspace):
    source_path = '/root/.internkim/state/flow.sqlite'
    destination_path = os.path.join(workspace, '.admind', 'flow.sqlite')
    if os.path.exists(destination_path):
        return
    source_exists = subprocess.run([CONTAINER, 'exec', name, 'test', '-f', source_path]).returncode == 0
    if not source_exists:
        return
    os.makedirs(os.path.dirname(destination_path), exist_ok=True)
    subprocess.run([CONTAINER, 'cp', f'{name}:{source_path}', destination_path], check=True)


def start_tenant(n, pg_ip, mm_ip):
    name = f'poc-tenant-{n:02d}'
    tenant_id = f'tenant_{n:02d}'
    team_name = f'tenant{n:02d}'
    secrets = os.path.join(BASE, 'secrets', tenant_id)
    config = os.path.join(BASE, 'config', tenant_id)
    workspace = os.path.join(BASE, 'workspace', tenant_id)
    openrouter = os.path.join(BASE, 'secrets', 'openrouter-key')
    os.makedirs(workspace, exist_ok=True)
    initialize_workspace_settings(workspace)

    existing = run([CONTAINER, 'inspect', name], check=False)
    if existing:
        migrate_flow_database(name, workspace)
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


def default_tenant_count():
    configured_count = os.environ.get('TENANT_COUNT')
    if configured_count:
        return int(configured_count)
    indices = []
    for config_path in glob.glob(os.path.join(BASE, 'config', 'tenant_*')):
        match = re.match(r'^tenant_([0-9]+)$', os.path.basename(config_path))
        if match:
            indices.append(int(match.group(1)))
    if indices:
        return max(indices)
    return FALLBACK_TENANT_COUNT


if __name__ == '__main__':
    count = int(sys.argv[1]) if len(sys.argv) > 1 else default_tenant_count()
    pg_ip = container_ip('poc-postgres')
    mm_ip = container_ip('poc-mattermost')
    print(f'Postgres: {pg_ip}, Mattermost: {mm_ip}')
    patch_ips_in_configs(pg_ip, mm_ip)
    for n in range(1, count + 1):
        start_tenant(n, pg_ip, mm_ip)
    print(f'All {count} tenants started')
