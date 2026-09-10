from pathlib import Path
import subprocess
import sys


workspace = Path('/mnt/shared/workspace')
evidence = Path(sys.argv[1])
evidence.mkdir(parents=True, exist_ok=True)


def run_acceptance(name, command, directory, expected_tests):
    log_path = evidence / f'{name}.log'
    with log_path.open('w') as output:
        result = subprocess.run(command, cwd=directory, stdout=output, stderr=subprocess.STDOUT, timeout=180)
    document = log_path.read_text()
    print(document, flush=True)
    if result.returncode != 0:
        raise RuntimeError(f'{name} failed; evidence is in {log_path}')
    for test_name in expected_tests:
        if not any(line.startswith(f'--- PASS: {test_name} (') for line in document.splitlines()):
            raise RuntimeError(f'{name} did not execute {test_name}')


database_test = 'TestTaskRetryAgainstPostgres'
run_acceptance('postgres-retry', [
    'sudo', '-u', 'postgres', 'env',
    'BLUECLAW_TEST_POSTGRES_URL=postgresql://postgres@/postgres?host=/var/run/postgresql&sslmode=disable',
    str(workspace / 'build/task-history-retry-postgres.test'),
    '-test.v', f'-test.run=^{database_test}$',
], workspace / '.dependency/blueclaw/internal/store/postgres', [database_test])

runtime_tests = [
    'TestRetryTaskRunQueuesOneChildAndPreservesSource',
    'TestRetryTaskRunRepairsEnqueueAfterTransientFailure',
    'TestRetryTaskRunResumesInterruptedChildWithoutDuplicateQueueLaunch',
    'TestRetryTaskRunResumesStartedChildThroughRuntimeRecovery',
    'TestRetryTaskRunRejectsForgedChildReference',
]
run_acceptance('runtime-retry', [
    str(workspace / 'build/task-history-retry-runtime.test'), '-test.v', '-test.run=^TestRetryTaskRun',
], workspace, runtime_tests)

run_acceptance('admind-retry', [
    str(workspace / 'build/task-history-retry-admind.test'), '-test.v', '-test.run=^TestTaskRunRetry',
], workspace, ['TestTaskRunRetryForwardsTrustedViewerAndPreservesBlueclawStatus'])

print('task-history-retry: passed', flush=True)
