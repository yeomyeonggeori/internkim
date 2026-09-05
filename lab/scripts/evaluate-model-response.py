import json
import pathlib
import sys
import time
import urllib.error
import urllib.request
import uuid


def read_json(path, document=None, transport=urllib.request.urlopen):
    body = None if document is None else json.dumps(document).encode()
    request = urllib.request.Request('http://127.0.0.1:8080' + path, data=body, headers={'Content-Type': 'application/json'})
    try:
        with transport(request, timeout=30) as response:
            return json.load(response)
    except urllib.error.HTTPError as error:
        body = error.read().decode(errors='replace').strip()
        raise RuntimeError(f'{request.get_method()} {path} returned HTTP {error.code}: {body}') from error


def find_task_run(task_runs, conversation_id):
    return next((task_run for task_run in task_runs if task_run.get('originConversationID') == conversation_id), None)


def evaluate_model_response(evidence_path, transport=urllib.request.urlopen, policy_path=pathlib.Path('/var/lib/blueclaw/delivery/config/policy.json'), monotonic=time.monotonic, sleep=time.sleep):
    evidence = {}
    try:
        policy = json.loads(policy_path.read_text())
        requester = next(person for person in policy['people'] if person.get('isAdmin'))
        sender_id = requester['emails'][0]
        message_id = str(uuid.uuid4())
        conversation_id = 'api:model-response-evaluation:' + str(uuid.uuid4())
        evidence['request'] = {
            'messageID': message_id,
            'conversationID': conversation_id,
            'senderID': sender_id,
        }
        ingress = read_json('/connectors/api/events', {
            'conversationID': conversation_id,
            'messageID': message_id,
            'senderID': sender_id,
            'replyTargetID': conversation_id,
            'prompt': '검증용 계산입니다. 도구를 사용하지 말고 173 + 289의 결과를 짧게 답해주세요.',
        }, transport)
        evidence['ingress'] = ingress
        deadline = monotonic() + 180
        detail = None
        while monotonic() < deadline:
            listed = read_json('/admin/api/run?viewerIsAdmin=true', transport=transport)
            task_run = find_task_run(listed, conversation_id)
            if task_run is None:
                sleep(1)
                continue
            task_run_id = task_run['taskRunID']
            detail = read_json('/admin/api/run/detail?taskRunID=' + task_run_id + '&viewerIsAdmin=true', transport=transport)
            evidence['liveAnswer'] = detail
            status = detail['taskRun']['status']
            if status == 'completed':
                break
            if status in ['failed', 'cancelled']:
                raise AssertionError(detail['taskRun'])
            sleep(1)
        assert detail is not None and detail['taskRun']['status'] == 'completed', detail
        assert detail['taskRun']['result'].strip(), detail['taskRun']
        requests = [event for event in detail['taskEvents'] if event['name'].startswith('tool.') and event['name'].endswith('.requested')]
        assert not requests, requests
        print('model-response-evaluation: task contract passed', flush=True)
    except Exception as error:
        evidence['evaluationError'] = {'type': type(error).__name__, 'message': str(error)}
        raise
    finally:
        evidence_path.parent.mkdir(parents=True, exist_ok=True)
        evidence_path.write_text(json.dumps(evidence, indent=2) + '\n')


if __name__ == '__main__':
    evaluate_model_response(pathlib.Path(sys.argv[1]))
