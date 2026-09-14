import json
import os
import subprocess
import sys
import uuid
from pathlib import Path


workspace = Path("/mnt/shared/workspace")
evidence = Path(sys.argv[1])
evidence.mkdir(parents=True, exist_ok=True)
database_name = "bluememo_acceptance_" + uuid.uuid4().hex


def run_acceptance(name, command, directory, environment, required_tests):
    log_path = evidence / (name + ".log")
    with log_path.open("w") as output:
        result = subprocess.run(command, cwd=directory, env=environment, stdout=output, stderr=subprocess.STDOUT, timeout=420)
    document = log_path.read_text()
    print(document, flush=True)
    if result.returncode != 0:
        raise RuntimeError(f"{name} failed with exit {result.returncode}; evidence: {log_path}")
    for test_name in required_tests:
        if not any(line.startswith(f"--- PASS: {test_name} (") for line in document.splitlines()):
            raise RuntimeError(f"{name} did not execute {test_name}")


subprocess.run(["sudo", "-u", "postgres", "createdb", database_name], check=True)
try:
    database_tests = [
        "TestApplyMigrationsSerializesConcurrentCalls",
        "TestApplyMigrationsRollsBackWhenLedgerInsertFails",
        "TestMigrationUpgradePreservesFactsAndLegacyProfiles",
        "TestPostgresJobClaimsFenceRequeuedAndExpiredWorkers",
        "TestConcurrentEpisodeReplayReturnsCanonicalReceipt",
        "TestRepeatedReinforcementSourceIncrementsOnce",
        "TestEpisodeAndForgetTransactionsRollBackWhenProfileQueueFails",
        "TestProfileFactsRespectPrivateOwnershipAndSharedAccess",
    ]
    run_acceptance("database", [
        "sudo", "-u", "postgres", "env",
        f"BLUEMEMO_TEST_POSTGRES_URL=postgresql://postgres@/{database_name}?host=/var/run/postgresql&sslmode=disable",
        str(workspace / "build/memory-postgres.test"), "-test.v",
    ], workspace, os.environ, database_tests)
    environment = {**os.environ,
        "BLUECLAW_E2E_LIVE": "1",
        "BLUECLAW_E2E_LLM_UNIX_SOCKET": "/run/internkim/capability.sock",
        "BLUECLAW_E2E_ARTIFACT_DIR": str(evidence / "model"),
        "BLUECLAW_SCENARIO_CAPABILITY_CATALOG": str(workspace / "pkg/capabilityprotocol/generated/capability-tools.json"),
        "BLUECLAW_SCENARIO_SKILL_ROOTS": str(workspace / ".dependency/internkim-plugin/skills"),
    }
    model_tests = ["TestBluememoIngestProfileAndReplayLive", "TestMemoryRecallWithoutConversationHistoryLive"]
    run_acceptance("model", [str(workspace / "build/memory-live.test"), "-test.v", "-test.run=^(" + "|".join(model_tests) + ")$"], workspace, environment, model_tests)
    (evidence / "result.json").write_text(json.dumps({"database": "passed", "liveModel": "passed"}, indent=2) + "\n")
finally:
    subprocess.run(["sudo", "-u", "postgres", "dropdb", "--if-exists", database_name], check=True)

print("memory-store: passed", flush=True)
