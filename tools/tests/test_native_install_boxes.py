import json
import re
import sqlite3
import sys
import tempfile
import unittest
import urllib.request
from pathlib import Path

repository_root = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(repository_root / "tools"))

import native_install_boxes as boxes  # noqa: E402

FIRST, SECOND, THIRD = boxes.OBSERVATIONS[:3]


def recorded(held, failed=()):
    return {**{name: True for name in held}, **{name: False for name in failed}}


class RatchetTests(unittest.TestCase):
    def test_a_run_whose_failures_are_exactly_the_list_passes(self):
        ratchet = boxes.judge(recorded([FIRST], [SECOND, THIRD]), (FIRST, SECOND, THIRD), {SECOND: "a", THIRD: "b"})
        self.assertTrue(ratchet.passes)
        self.assertEqual(ratchet.held, (FIRST,))
        self.assertEqual(ratchet.not_yet, (SECOND, THIRD))

    def test_a_failure_the_list_does_not_name_fails_the_run(self):
        ratchet = boxes.judge(recorded([], [FIRST, SECOND]), (FIRST, SECOND), {SECOND: "a"})
        self.assertFalse(ratchet.passes)
        self.assertEqual(ratchet.unlisted_failures, (FIRST,))

    def test_a_listed_observation_that_holds_fails_the_run_so_the_list_only_shrinks(self):
        ratchet = boxes.judge(recorded([FIRST, SECOND]), (FIRST, SECOND), {SECOND: "a"})
        self.assertFalse(ratchet.passes)
        self.assertEqual(ratchet.listed_but_held, (SECOND,))

    def test_an_observation_the_run_never_reached_did_not_hold(self):
        ratchet = boxes.judge(recorded([FIRST]), (FIRST, SECOND), {})
        self.assertFalse(ratchet.passes)
        self.assertEqual(ratchet.unlisted_failures, (SECOND,))
        self.assertTrue(boxes.judge(recorded([FIRST]), (FIRST, SECOND), {SECOND: "a"}).passes)

    def test_an_observation_recorded_under_a_name_nobody_listed_fails_the_run(self):
        ratchet = boxes.judge(recorded([FIRST, "a sentence somebody reworded"]), (FIRST,), {})
        self.assertFalse(ratchet.passes)
        self.assertEqual(ratchet.unnamed, ("a sentence somebody reworded",))

    def test_the_report_counts_what_holds_and_what_is_not_yet_and_names_them(self):
        not_yet = {SECOND: "seeding step 4"}
        report = boxes.ratchet_report(boxes.judge(recorded([FIRST], [SECOND]), (FIRST, SECOND), not_yet), not_yet)
        self.assertTrue(report.startswith("1 hold, 1 not yet"))
        self.assertIn(f"not yet: {SECOND} (seeding step 4)", report)

    def test_every_observation_has_one_name_and_the_list_names_only_observations(self):
        self.assertEqual(len(boxes.OBSERVATIONS), len(set(boxes.OBSERVATIONS)))
        self.assertEqual(sorted(set(boxes.NOT_YET) - set(boxes.OBSERVATIONS)), [])
        self.assertTrue(all(reason.strip() for reason in boxes.NOT_YET.values()))

    def test_the_scenario_records_only_names_the_ratchet_knows(self):
        driver = (repository_root / "tools" / "test-native-install").read_text()
        constants = {name for name in re.findall(r"boxes\.([A-Z_]+)", driver) if isinstance(getattr(boxes, name, None), str)}
        named = {getattr(boxes, name) for name in constants if getattr(boxes, name) in boxes.OBSERVATIONS}
        self.assertEqual(sorted(set(boxes.OBSERVATIONS) - named - {boxes.INSTALLED_ON_A}), [])


class WriterTests(unittest.TestCase):
    def test_rows_are_kept_when_they_are_a_gap_free_prefix_through_every_acknowledgement(self):
        self.assertTrue(boxes.sequence_kept([1, 2, 3], [1, 2, 3])[0])
        self.assertTrue(boxes.sequence_kept([1, 2, 3, 4], [1, 2, 3])[0])
        self.assertFalse(boxes.sequence_kept([1, 2, 3, 4, 5], [1, 2, 3])[0])
        self.assertFalse(boxes.sequence_kept([1, 3, 4], [1, 2, 3])[0])
        self.assertFalse(boxes.sequence_kept([1, 2], [1, 2, 3])[0])

    def test_files_are_kept_with_their_content_and_at_most_one_unacknowledged_write(self):
        written = {index: boxes.file_content(index) for index in (1, 2, 3)}
        self.assertTrue(boxes.file_writes_kept(written, [1, 2])[0])
        self.assertFalse(boxes.file_writes_kept(written, [1])[0])
        self.assertFalse(boxes.file_writes_kept({**written, 2: "torn"}, [1, 2, 3])[0])
        self.assertFalse(boxes.file_writes_kept({1: boxes.file_content(1)}, [1, 2])[0])

    def test_every_acknowledged_message_must_be_stored(self):
        self.assertTrue(boxes.messages_kept(["a", "b", "c"], ["a", "b"])[0])
        self.assertFalse(boxes.messages_kept(["a"], ["a", "b"])[0])

    def test_the_writers_ask_the_barrier_in_the_words_it_answers(self):
        source = (repository_root / "internal" / "filestream" / "barrier_socket.go").read_text()
        words = dict(re.findall(r'(barrier\w+)\s+=\s+"([^"]*)"', source))
        self.assertEqual(words["barrierRequest"], boxes.BARRIER_REQUEST)
        self.assertEqual(words["barrierKept"], boxes.BARRIER_KEPT)

    def test_an_acknowledgement_is_kept_on_this_machine_with_its_time(self):
        with tempfile.TemporaryDirectory() as directory:
            log = Path(directory) / "acknowledgements.log"
            acknowledgements = boxes.Acknowledgements(log, "127.0.0.1")
            try:
                for index in (1, 2):
                    urllib.request.urlopen(f"{acknowledgements.url}/database/power-cut/{index}", timeout=5).read()
                self.assertEqual(acknowledgements.of("database", "power-cut"), [1, 2])
                self.assertEqual(len(log.read_text().splitlines()), 2)
            finally:
                acknowledgements.stop_the_messages()

    def test_the_longest_wait_between_messages_counts_from_when_the_move_began(self):
        with tempfile.TemporaryDirectory() as directory:
            acknowledgements = boxes.Acknowledgements(Path(directory) / "log", "127.0.0.1")
            try:
                acknowledgements.received = [(110.0, "messages", "move", "a"), (150.0, "messages", "move", "b"), (155.0, "messages", "move", "c")]
                self.assertEqual(acknowledgements.longest_message_gap("move", 100.0), 40.0)
                self.assertEqual(acknowledgements.longest_message_gap("move", 200.0), float("inf"))
            finally:
                acknowledgements.stop_the_messages()


class ShareTests(unittest.TestCase):
    def test_the_log_is_the_store_s_pieces_of_both_logs_under_the_company(self):
        stored = ["company/wal/7/a", "company/file-chunks/b", "company/snapshots/x/manifest.json", "other/wal/7/c"]
        self.assertEqual(boxes.pieces_of_the_log(stored, "company"), ["file-chunks/b", "wal/7/a"])

    def test_the_standbys_split_the_pieces_with_none_left_out_and_none_twice(self):
        pieces = ["a", "b", "c", "d"]
        self.assertTrue(boxes.shares_split(pieces, {"B": {"a", "b"}, "C": {"c", "d"}})[0])
        self.assertFalse(boxes.shares_split(pieces, {"B": {"a", "b"}, "C": {"c"}})[0])
        self.assertFalse(boxes.shares_split(pieces, {"B": {"a", "b", "c"}, "C": {"c", "d"}})[0])
        self.assertFalse(boxes.shares_split([], {"B": set(), "C": set()})[0])

    def test_a_reported_share_is_even_within_ten_points(self):
        self.assertTrue(boxes.shares_reported_evenly({"B": 0.45, "C": 0.55})[0])
        self.assertFalse(boxes.shares_reported_evenly({"B": 0.35, "C": 0.65})[0])
        self.assertFalse(boxes.shares_reported_evenly({"B": None, "C": 0.5})[0])
        self.assertFalse(boxes.shares_reported_evenly({})[0])


class StateTests(unittest.TestCase):
    def test_only_the_box_directory_and_the_session_key_are_exempt(self):
        before = {"state\tbox/identity.json": "a", "configuration\tagent-key": "a", "state\tcompanies/x/secrets/agent-key": "a", "workspace\tshared/note": "a", "relay\tqueue": "a"}
        after = {"state\tbox/identity.json": "b", "configuration\tagent-key": "b", "state\tcompanies/x/secrets/agent-key": "b", "workspace\tshared/note": "b"}
        self.assertEqual(boxes.compare_to_the_state_read(before, after), ["missing relay/queue: a", "differs workspace/shared/note: a -> b"])

    def test_a_database_is_compared_by_its_dump_and_its_side_files_through_it(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "memory.db"
            connection = sqlite3.connect(path)
            connection.execute("create table fact (text)")
            connection.executemany("insert into fact values (?)", [("x" * 500,)] * 50)
            connection.commit()
            connection.execute("delete from fact where rowid > 1")
            connection.commit()
            dumped = boxes.digest_of(str(path))
            connection.execute("vacuum")
            connection.close()
            self.assertTrue(dumped.startswith("dump:"))
            self.assertEqual(boxes.digest_of(str(path)), dumped)
            (Path(directory) / "memory.db-wal").write_bytes(b"")
            self.assertTrue(boxes.is_beside_a_database(str(path) + "-wal"))
            self.assertFalse(boxes.is_beside_a_database(str(path)))

    def test_a_connection_file_is_compared_without_the_session_it_carries(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "connection.json"
            path.write_text(json.dumps({"company": {"id": "x"}, "agentKey": "first"}))
            first = boxes.digest_of(str(path))
            path.write_text(json.dumps({"agentKey": "second", "company": {"id": "x"}}))
            self.assertEqual(boxes.digest_of(str(path)), first)
            path.write_text(json.dumps({"agentKey": "second", "company": {"id": "y"}}))
            self.assertNotEqual(boxes.digest_of(str(path)), first)


if __name__ == "__main__":
    unittest.main()
