import json
import sys
import tempfile
import unittest
from pathlib import Path

repository_root = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(repository_root / "tools"))

import native_install_deliveries as deliveries  # noqa: E402

EXAMPLE_DIRECTORY = repository_root / "tools" / "native-install-requests-example"


class RequestBatchTests(unittest.TestCase):
    def write_batch(self, document):
        directory = Path(self.enterContext(tempfile.TemporaryDirectory()))
        path = directory / "requests.json"
        path.write_text(json.dumps(document))
        return path

    def test_the_example_batch_loads_with_its_attachment_resolved(self):
        requests = deliveries.load_requests(EXAMPLE_DIRECTORY / "requests.json")
        self.assertEqual([request["name"] for request in requests], ["weekly-report", "summarize-notes"])
        self.assertEqual(requests[0]["attachments"], [])
        attachment = requests[1]["attachments"][0]
        self.assertEqual(attachment["filename"], "sample-notes.txt")
        self.assertEqual(attachment["contentType"], "text/plain")
        self.assertTrue(Path(attachment["path"]).is_file())

    def test_an_attachment_may_name_its_content_type(self):
        described = {"path": "slides/deck.bin", "contentType": "application/vnd.ms-powerpoint"}
        file = deliveries.attachment_file(Path("/batch"), described)
        self.assertEqual(file["path"], "/batch/slides/deck.bin")
        self.assertEqual(file["contentType"], "application/vnd.ms-powerpoint")

    def test_an_unknown_extension_is_sent_as_bytes(self):
        file = deliveries.attachment_file(Path("/batch"), "notes.unknownextension")
        self.assertEqual(file["contentType"], "application/octet-stream")

    def test_a_batch_that_is_not_a_list_is_refused(self):
        with self.assertRaises(ValueError):
            deliveries.load_requests(self.write_batch({"name": "a", "text": "b"}))

    def test_a_name_that_would_leave_the_deliveries_directory_is_refused(self):
        for name in ("../escape", "nested/name", "", ".."):
            with self.subTest(name=name), self.assertRaises(ValueError):
                deliveries.load_requests(self.write_batch([{"name": name, "text": "이샘플"}]))

    def test_a_request_without_text_is_refused(self):
        with self.assertRaises(ValueError):
            deliveries.load_requests(self.write_batch([{"name": "empty", "text": "  "}]))

    def test_the_snippet_defaults_to_the_first_line_of_the_text(self):
        self.assertEqual(deliveries.first_line_snippet({"text": "\n이샘플 보고서\n둘째 줄"}), "이샘플 보고서")
        self.assertEqual(deliveries.first_line_snippet({"text": "x" * 80}), "x" * 40)
        self.assertEqual(deliveries.first_line_snippet({"text": "이샘플", "snippet": "박예시"}), "박예시")


class DeliveredMediaTests(unittest.TestCase):
    def test_media_is_read_from_imeta_tags_that_carry_a_url(self):
        tags = [
            ["p", "someone"],
            ["imeta", "url http://example.com/a/report.docx", "x abc", "size 12", "m text/plain"],
            ["imeta", "x without-a-url"],
        ]
        self.assertEqual(
            deliveries.media_of(tags),
            [{"url": "http://example.com/a/report.docx", "x": "abc", "size": "12", "m": "text/plain"}],
        )

    def test_a_reply_without_tags_carries_no_media(self):
        self.assertEqual(deliveries.media_of(None), [])


class WorkspaceSnapshotTests(unittest.TestCase):
    def test_the_command_skips_what_the_reply_carried_and_the_service_state(self):
        command = deliveries.workspace_snapshot_command("/srv/internkim-rig/deliveries/a b/workspace", 1700000000.9, ["bbb", "aaa", ""])
        self.assertIn("'/srv/internkim-rig/deliveries/a b/workspace'", command)
        self.assertIn('skipped=" aaa bbb "', command)
        self.assertIn("-newermt @1700000000", command)
        self.assertIn("-not -path '/workspace/.blueclaw/*'", command)
        self.assertIn("node_modules", command)

    def test_kept_paths_are_read_from_the_commands_output(self):
        output = "noise\nkept=workspace/private/people/a/documents/보고서.docx\nkept=workspace/shared/x.md\n"
        self.assertEqual(
            deliveries.kept_paths(output),
            ["workspace/private/people/a/documents/보고서.docx", "workspace/shared/x.md"],
        )


if __name__ == "__main__":
    unittest.main()
