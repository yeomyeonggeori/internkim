import json
import sys
import tempfile
import unittest
from pathlib import Path

repository_root = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(repository_root / "tools"))

import e2e_tasks  # noqa: E402

E2E_TASKS_DIRECTORY = repository_root / "web" / "tests" / "e2e" / "tasks"


class TaskDirectoryTests(unittest.TestCase):
    def write_task(self, document):
        directory = Path(self.enterContext(tempfile.TemporaryDirectory()))
        (directory / "01-task.json").write_text(json.dumps(document))
        return directory

    def task(self, **fields):
        return {
            "name": "a",
            "requesterEmail": "member3@example.com",
            "instruction": "이샘플",
            "assertions": [{"replyMentions": ["이샘플"]}],
            **fields,
        }

    def test_every_e2e_task_loads(self):
        tasks = e2e_tasks.load_tasks(E2E_TASKS_DIRECTORY)
        self.assertGreaterEqual(len(tasks), 30)

    def test_only_runs_the_named_tasks(self):
        tasks = e2e_tasks.load_tasks(E2E_TASKS_DIRECTORY, only="remote-work-clock-in,message-a-colleague")
        self.assertEqual([task["name"] for task in tasks], ["remote-work-clock-in", "message-a-colleague"])

    def test_only_refuses_a_name_no_task_carries(self):
        with self.assertRaises(ValueError):
            e2e_tasks.load_tasks(E2E_TASKS_DIRECTORY, only="remote-work-clock-in,nowhere")

    def test_a_name_that_would_leave_the_deliveries_directory_is_refused(self):
        for name in ("../escape", "nested/name", "", ".."):
            with self.subTest(name=name), self.assertRaises(ValueError):
                e2e_tasks.load_tasks(self.write_task(self.task(name=name)))

    def test_a_task_without_a_requester_an_instruction_or_an_assertion_is_refused(self):
        for missing in ({"requesterEmail": ""}, {"instruction": "  "}, {"assertions": []}):
            with self.subTest(missing=missing), self.assertRaises(ValueError):
                e2e_tasks.load_tasks(self.write_task(self.task(**missing)))

    def test_an_attachment_resolves_beside_the_tasks(self):
        directory = self.write_task(self.task(attachments=["notes.txt"]))
        attachment = e2e_tasks.load_tasks(directory)[0]["attachments"][0]
        self.assertEqual(attachment["path"], str((directory / "notes.txt").resolve()))
        self.assertEqual(attachment["contentType"], "text/plain")

    def test_an_attachment_may_name_its_content_type(self):
        described = {"path": "slides/deck.bin", "contentType": "application/vnd.ms-powerpoint"}
        file = e2e_tasks.attachment_file(Path("/batch"), described)
        self.assertEqual(file["path"], "/batch/slides/deck.bin")
        self.assertEqual(file["contentType"], "application/vnd.ms-powerpoint")

    def test_an_unknown_extension_is_sent_as_bytes(self):
        file = e2e_tasks.attachment_file(Path("/batch"), "notes.unknownextension")
        self.assertEqual(file["contentType"], "application/octet-stream")


class FollowingATaskTests(unittest.TestCase):
    def test_the_latest_task_names_the_outcome(self):
        self.assertEqual(e2e_tasks.status_of_latest([{"status": "completed"}, {"status": "waiting_approval"}]), "waiting_approval")
        self.assertEqual(e2e_tasks.status_of_latest([]), "no task")

    def test_a_task_still_holding_its_question_has_not_moved(self):
        held = [{"task_run_id": "a", "status": "waiting_approval", "updated_at": "2026-11-06T04:48:22+00:00"}]
        self.assertEqual(e2e_tasks.ledger_state(held), e2e_tasks.ledger_state([dict(held[0])]))

    def test_a_run_the_answer_started_moves_the_ledger(self):
        held = [{"task_run_id": "a", "status": "waiting_approval", "updated_at": "2026-11-06T04:48:22+00:00"}]
        resumed = held + [{"task_run_id": "b", "status": "completed", "updated_at": "2026-11-06T04:49:26+00:00"}]
        self.assertNotEqual(e2e_tasks.ledger_state(resumed), e2e_tasks.ledger_state(held))


class DeliveredMediaTests(unittest.TestCase):
    def test_media_is_read_from_imeta_tags_that_carry_a_url(self):
        tags = [
            ["p", "someone"],
            ["imeta", "url http://example.com/a/report.docx", "x abc", "size 12", "m text/plain"],
            ["imeta", "x without-a-url"],
        ]
        self.assertEqual(
            e2e_tasks.media_of(tags),
            [{"url": "http://example.com/a/report.docx", "x": "abc", "size": "12", "m": "text/plain"}],
        )

    def test_a_reply_without_tags_carries_no_media(self):
        self.assertEqual(e2e_tasks.media_of(None), [])


class WorkspaceSnapshotTests(unittest.TestCase):
    def test_the_command_skips_what_the_reply_carried_and_the_service_state(self):
        command = e2e_tasks.workspace_snapshot_command("/srv/internkim-rig/deliveries/a b/workspace", "2026-11-06T05:46:23.722Z", ["bbb", "aaa", ""])
        self.assertIn("'/srv/internkim-rig/deliveries/a b/workspace'", command)
        self.assertIn('skipped=" aaa bbb "', command)
        self.assertIn("-newermt 2026-11-06T05:46:23.722Z", command)
        self.assertIn("-not -path '/workspace/.blueclaw/*'", command)
        self.assertIn("node_modules", command)

    def test_kept_paths_are_read_from_the_commands_output(self):
        output = "noise\nkept=workspace/private/people/a/documents/보고서.docx\nkept=workspace/shared/x.md\n"
        self.assertEqual(
            e2e_tasks.kept_paths(output),
            ["workspace/private/people/a/documents/보고서.docx", "workspace/shared/x.md"],
        )


if __name__ == "__main__":
    unittest.main()
