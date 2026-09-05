import importlib.util
import json
import pathlib
import tempfile
import unittest
import urllib.error
import urllib.parse


script_path = pathlib.Path(__file__).parents[2] / 'lab/scripts/evaluate-model-response.py'
module_spec = importlib.util.spec_from_file_location('evaluate_model_response', script_path)
evaluate_model_response = importlib.util.module_from_spec(module_spec)
module_spec.loader.exec_module(evaluate_model_response)


class FakeResponse:
    def __init__(self, document):
        self.document = document

    def __enter__(self):
        return self

    def __exit__(self, exception_type, exception, traceback):
        return False

    def read(self):
        return json.dumps(self.document).encode()


class FakeTransport:
    def __init__(self, task_runs=None, detail=None, ingress_error=None):
        self.task_runs = task_runs
        self.detail = detail
        self.ingress_error = ingress_error
        self.requests = []

    def __call__(self, request, timeout):
        self.requests.append(request)
        if len(self.requests) == 1:
            if self.ingress_error:
                raise self.ingress_error
            return FakeResponse({'handled': True, 'reason': 'queued'})
        if len(self.requests) == 2:
            conversation_id = json.loads(self.requests[0].data)['conversationID']
            return FakeResponse(self.task_runs(conversation_id) if callable(self.task_runs) else self.task_runs or [])
        return FakeResponse(self.detail)


def policy_path():
    directory = tempfile.TemporaryDirectory()
    path = pathlib.Path(directory.name) / 'policy.json'
    path.write_text(json.dumps({'people': [{'isAdmin': True, 'emails': ['admin@example.com']}] }))
    return directory, path


class EvaluateModelResponseTests(unittest.TestCase):
    def test_sends_one_ingress_and_matches_exact_conversation(self):
        directory, path = policy_path()
        self.addCleanup(directory.cleanup)
        transport = FakeTransport(
            task_runs=lambda conversation_id: [
                {'taskRunID': 'other', 'originConversationID': conversation_id + '-other'},
                {'taskRunID': 'task-1', 'originConversationID': conversation_id},
            ],
            detail={'taskRun': {'taskRunID': 'task-1', 'status': 'completed', 'result': '결과'}, 'taskEvents': []},
        )
        with tempfile.TemporaryDirectory() as evidence_directory:
            evidence_path = pathlib.Path(evidence_directory) / 'evidence.json'
            evaluate_model_response.evaluate_model_response(evidence_path, transport, path, monotonic=lambda: 0, sleep=lambda _: None)
            self.assertEqual(len(transport.requests), 3)
            evidence = json.loads(evidence_path.read_text())
            self.assertEqual(evidence['request']['conversationID'], json.loads(transport.requests[0].data)['conversationID'])
            detail_query = urllib.parse.parse_qs(urllib.parse.urlparse(transport.requests[2].full_url).query)
            self.assertEqual(detail_query['taskRunID'], ['task-1'])

    def test_times_out_without_resubmitting_and_preserves_request_ids(self):
        directory, path = policy_path()
        self.addCleanup(directory.cleanup)
        transport = FakeTransport(task_runs=[])
        with tempfile.TemporaryDirectory() as evidence_directory:
            evidence_path = pathlib.Path(evidence_directory) / 'evidence.json'
            with self.assertRaises(AssertionError):
                evaluate_model_response.evaluate_model_response(evidence_path, transport, path, monotonic=iter([0, 0, 181]).__next__, sleep=lambda _: None)
            evidence = json.loads(evidence_path.read_text())
            self.assertIn('conversationID', evidence['request'])
            self.assertEqual(len(transport.requests), 2)

    def test_collects_failed_ledger_and_marks_evaluation_failed(self):
        directory, path = policy_path()
        self.addCleanup(directory.cleanup)
        transport = FakeTransport(
            task_runs=lambda conversation_id: [{'taskRunID': 'task-1', 'originConversationID': conversation_id}],
            detail={'taskRun': {'taskRunID': 'task-1', 'status': 'failed'}, 'taskEvents': [{'name': 'task.failed'}]},
        )
        with tempfile.TemporaryDirectory() as evidence_directory:
            evidence_path = pathlib.Path(evidence_directory) / 'evidence.json'
            with self.assertRaises(AssertionError):
                evaluate_model_response.evaluate_model_response(evidence_path, transport, path, monotonic=lambda: 0, sleep=lambda _: None)
            evidence = json.loads(evidence_path.read_text())
            self.assertEqual(evidence['liveAnswer']['taskRun']['status'], 'failed')
            self.assertEqual(evidence['evaluationError']['type'], 'AssertionError')
            self.assertEqual(len(transport.requests), 3)

    def test_rejects_tool_requests(self):
        directory, path = policy_path()
        self.addCleanup(directory.cleanup)
        transport = FakeTransport(
            task_runs=lambda conversation_id: [{'taskRunID': 'task-1', 'originConversationID': conversation_id}],
            detail={'taskRun': {'taskRunID': 'task-1', 'status': 'completed', 'result': '결과'}, 'taskEvents': [{'name': 'tool.x.requested'}]},
        )
        with tempfile.TemporaryDirectory() as evidence_directory:
            evidence_path = pathlib.Path(evidence_directory) / 'evidence.json'
            with self.assertRaises(AssertionError):
                evaluate_model_response.evaluate_model_response(evidence_path, transport, path, monotonic=lambda: 0, sleep=lambda _: None)
            evidence = json.loads(evidence_path.read_text())
            self.assertEqual(evidence['evaluationError']['type'], 'AssertionError')

    def test_preserves_http_rejection_body(self):
        directory, path = policy_path()
        self.addCleanup(directory.cleanup)
        rejection = urllib.error.HTTPError('http://127.0.0.1:8080/connectors/api/events', 409, 'Conflict', {}, FakeResponseBody('connector disabled'))
        transport = FakeTransport(ingress_error=rejection)
        with tempfile.TemporaryDirectory() as evidence_directory:
            evidence_path = pathlib.Path(evidence_directory) / 'evidence.json'
            with self.assertRaisesRegex(RuntimeError, 'POST /connectors/api/events returned HTTP 409: connector disabled'):
                evaluate_model_response.evaluate_model_response(evidence_path, transport, path)
            self.assertIn('connector disabled', json.loads(evidence_path.read_text())['evaluationError']['message'])

    def test_records_ids_when_ingress_times_out_without_resubmitting(self):
        directory, path = policy_path()
        self.addCleanup(directory.cleanup)
        transport = FakeTransport(ingress_error=TimeoutError('write timed out'))
        with tempfile.TemporaryDirectory() as evidence_directory:
            evidence_path = pathlib.Path(evidence_directory) / 'evidence.json'
            with self.assertRaisesRegex(TimeoutError, 'write timed out'):
                evaluate_model_response.evaluate_model_response(evidence_path, transport, path)
            evidence = json.loads(evidence_path.read_text())
            self.assertIn('messageID', evidence['request'])
            self.assertIn('conversationID', evidence['request'])
            self.assertEqual(len(transport.requests), 1)


class FakeResponseBody:
    def __init__(self, body):
        self.body = body

    def read(self):
        return self.body.encode()

    def close(self):
        return None


if __name__ == '__main__':
    unittest.main()
