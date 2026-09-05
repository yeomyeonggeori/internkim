import json
import pathlib
import subprocess
import sys
import time
import urllib.error
import urllib.request


def read_health():
    try:
        with urllib.request.urlopen('http://127.0.0.1:8080/admin/api/health', timeout=5) as response:
            return response.status, json.load(response)
    except urllib.error.HTTPError as error:
        return error.code, json.load(error)


def wait_for_health(phase, predicate):
    started = time.monotonic()
    deadline = started + 240
    last_response = None
    print(phase + ': waiting', flush=True)
    while time.monotonic() < deadline:
        try:
            last_response = read_health()
            evidence[phase] = {'elapsedSeconds': round(time.monotonic() - started, 2), 'status': last_response[0], 'health': last_response[1]}
            if predicate(*last_response):
                print(phase + ': ready', flush=True)
                return last_response
        except (urllib.error.URLError, TimeoutError, ConnectionError):
            pass
        time.sleep(1)
    raise RuntimeError(phase + ' did not reach the expected state: ' + str(last_response))


def restart(service):
    subprocess.run(['systemctl', 'restart', service], check=True, timeout=90)


def write_document(path, document):
    path.write_text(json.dumps(document, indent=2) + '\n')


paths = [
    pathlib.Path('/root/.blueclaw/config/runtime.json'),
    pathlib.Path('/root/.blueclaw/workspace/.blueclaw/config/runtime.json'),
    pathlib.Path('/var/lib/blueclaw/delivery/config/runtime.json'),
]
evidence_path = pathlib.Path(sys.argv[1])
evidence_path.parent.mkdir(parents=True, exist_ok=True)
originals = {path: path.read_text() for path in paths}
evidence = {}
try:
    wait_for_health('initialDependencies', lambda status, health: status == 200 and health['status'] == 'ok')
    invalid = json.loads(originals[paths[-1]])
    invalid['languageModel']['capability'].pop('maxModel')
    write_document(paths[-1], invalid)
    restart('blueclaw')
    status, health = wait_for_health('invalidApplicationStartup', lambda status, health: 'database' in health)
    evidence['invalidConfigurationHealth'] = health
    assert status == 503, ('A missing model tier passed health', status, health)
    assert not health['languageModel']['configured'], health
    assert 'max tier no model' in health['languageModel']['error'], health
    assert not health['connector']['started'], health
    request = urllib.request.Request('http://127.0.0.1:8080/admin/api/run/start', data=b'{}', headers={'Content-Type': 'application/json'})
    try:
        urllib.request.urlopen(request, timeout=5)
    except urllib.error.HTTPError as error:
        assert error.code == 503 and 'quiesced' in error.read().decode(), error
    else:
        raise AssertionError('Invalid model configuration admitted a task')
    for path in paths:
        legacy = json.loads(originals[path])
        language_model = legacy['languageModel']
        capability = language_model['capability']
        language_model['defaultProvider'] = 'capabilityLLM'
        language_model['fallbackProvider'] = ''
        language_model.pop('embedding', None)
        for field in list(capability):
            if field.endswith('Model') and field != 'lowModel':
                del capability[field]
        write_document(path, legacy)
    restart('internkim-admind')
    wait_for_health('migratedApplicationStartup', lambda status, health: 'database' in health)
    status, health = wait_for_health('migratedDependencies', lambda status, health: status == 200 and health['status'] == 'ok')
    evidence['migratedConfigurationHealth'] = health
    assert health['languageModel']['configured'], health
    for path in paths:
        migrated = json.loads(path.read_text())['languageModel']
        original = json.loads(originals[path])['languageModel']
        assert 'defaultProvider' not in migrated, str(path)
        assert migrated['embedding']['model'], str(path)
        assert migrated['capability']['lowModel'] == original['capability']['lowModel'], str(path)
        for field in original['capability']:
            if field.endswith('Model'):
                assert migrated['capability'].get(field), (str(path), field)
    evidence['preservedExplicitTier'] = True
    evidence['allConfigurationCopiesMigrated'] = True
    print('model-configuration-upgrade: ok', flush=True)
finally:
    evidence_path.write_text(json.dumps(evidence, indent=2) + '\n')
    for path, original in originals.items():
        path.write_text(original)
    restart('blueclaw')
