import json
import re
import sqlite3
import sys
import tempfile
import threading
import time
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

    def test_the_wait_across_a_move_runs_from_the_last_message_before_it_to_the_first_once_the_box_serves(self):
        landed = [72.0, 80.0, 95.0, 130.0, 141.0, 150.0]
        self.assertEqual(boxes.move_interruption(landed, 100.0, 140.0, 30), 35.0)
        self.assertEqual(boxes.move_interruption(landed, 100.0, 120.0, 30), 35.0)

    def test_a_wait_across_a_move_cannot_be_told_when_no_message_landed_just_before_it_or_after_it(self):
        self.assertIsNone(boxes.move_interruption([10.0, 130.0, 141.0], 100.0, 120.0, 30))
        self.assertIsNone(boxes.move_interruption([80.0, 95.0], 100.0, 120.0, 30))
        self.assertIsNone(boxes.move_interruption([80.0, 95.0, 130.0], 100.0, None, 30))

    def test_a_round_trip_is_the_median_of_the_messages_that_landed_and_reasons_are_counted(self):
        attempts = [
            boxes.MessageAttempt("move", 1, 0.0, 1.0, "a", ""),
            boxes.MessageAttempt("move", 2, 1.0, 4.0, "b", ""),
            boxes.MessageAttempt("move", 3, 4.0, 19.0, "", "0 never answered"),
            boxes.MessageAttempt("move", 3, 20.0, 35.0, "", "0 never answered"),
            boxes.MessageAttempt("move", 3, 36.0, 37.0, "c", ""),
        ]
        self.assertEqual(boxes.message_round_trip(attempts), 1.0)
        self.assertIsNone(boxes.message_round_trip(attempts[2:4]))
        self.assertEqual(boxes.unanswered_reasons(attempts, 0.0, 40.0), ["2x 0 never answered"])

    def test_pausing_the_messages_waits_for_the_one_in_flight_and_sends_no_more(self):
        with tempfile.TemporaryDirectory() as directory:
            acknowledgements = boxes.Acknowledgements(Path(directory) / "log", "127.0.0.1")
            in_flight = threading.Event()
            answered = threading.Event()

            def send(index):
                in_flight.set()
                answered.wait(5)
                return f"message-{index}", ""

            try:
                acknowledgements.keep_sending_messages(send, "power-cut")
                in_flight.wait(5)
                threading.Timer(0.3, answered.set).start()
                acknowledgements.pause_the_messages()
                sent = acknowledgements.of("messages", "power-cut")
                time.sleep(0.5)
                self.assertGreaterEqual(len(sent), 1)
                self.assertEqual(acknowledgements.of("messages", "power-cut"), sent)
            finally:
                acknowledgements.stop_the_messages()


class ShareTests(unittest.TestCase):
    def test_the_log_is_the_store_s_pieces_of_both_logs_under_the_company(self):
        stored = {"company/wal/7/a": "1", "company/file-chunks/b": "2", "company/snapshots/x/manifest.json": "3", "other/wal/7/c": "4"}
        self.assertEqual(boxes.pieces_of_the_log(stored, "company"), {"file-chunks/b": "2", "wal/7/a": "1"})

    def test_a_piece_kept_after_a_standby_last_listed_the_log_is_not_yet_due_on_it(self):
        pieces = {"a": "2026-11-24T08:05:00.100Z", "b": "2026-11-24T08:05:01.200Z", "c": "2026-11-24T08:05:09.000Z"}
        kept_through = {"B": "2026-11-24T08:05:01.200+00:00", "C": "2026-11-24T08:05:05.000+00:00"}
        self.assertTrue(boxes.shares_split_through(pieces, kept_through, {"B": {"a"}, "C": {"b"}})[0])
        self.assertFalse(boxes.shares_split_through(pieces, kept_through, {"B": {"a"}, "C": set()})[0])
        self.assertFalse(boxes.shares_split_through(pieces, {"B": None, "C": "2026-11-24T08:05:05.000+00:00"}, {"B": {"a"}, "C": {"b", "c"}})[0])

    def test_the_moment_a_standby_reports_is_compared_to_the_millisecond_its_listing_gives(self):
        self.assertTrue(boxes.shares_split_through({"a": "2026-11-24T08:05:01.200731+00:00"}, {"B": "2026-11-24T08:05:01.200+00:00"}, {"B": {"a"}})[0])

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

    def test_the_media_store_s_locks_and_the_barrier_s_markers_are_exempt(self):
        before = {"state\tmedia/.vgwlocks/x/09": "a", "state\t.file-journal-barrier/m": "a", "state\tmedia/buzz-media/packs/p": "a"}
        after = {"state\tmedia/.vgwlocks/x/17": "a", "state\tmedia/buzz-media/packs/p": "b"}
        self.assertEqual(boxes.compare_to_the_state_read(before, after), ["differs state/media/buzz-media/packs/p: a -> b"])

    def test_an_object_the_relay_s_probe_adds_is_not_counted_and_one_it_loses_is(self):
        probe = "type=f digest=x written-by=relay-probe"
        before = {"state\tmedia/buzz-media/packs/old": probe}
        after = {"state\tmedia/buzz-media/packs/new": probe, "state\tmedia/buzz-media/packs/real": "type=f digest=y"}
        self.assertEqual(
            boxes.compare_to_the_state_read(before, after),
            ["missing state/media/buzz-media/packs/old: " + probe, "extra state/media/buzz-media/packs/real: type=f digest=y"],
        )

    def test_a_probe_object_is_known_by_the_body_the_relay_s_probe_writes(self):
        with tempfile.TemporaryDirectory() as directory:
            for name, body, is_probe in (
                ("sequential", b"probe-sequential-0b6f1b4e-6a5e-4f8e-9d3c-2f1a7b9c8d01-2", True),
                ("race", b"probe-inm-race-0b6f1b4e-6a5e-4f8e-9d3c-2f1a7b9c8d01-0", True),
                ("etag", b"probe-etag-1-0b6f1b4e-6a5e-4f8e-9d3c-2f1a7b9c8d01", True),
                ("pointer seed", b"probe-pointer-seed", True),
                ("pointer", b"round=2,racer=17,nonce=0b6f1b4e-6a5e-4f8e-9d3c-2f1a7b9c8d01", True),
                ("pointer of another shape", b"round=2,racer=17,nonce=someone", False),
                ("pack", b"PACK\x00\x00\x00\x02probe-sequential-", False),
                ("longer", b"probe-sequential-0b6f1b4e-6a5e-4f8e-9d3c-2f1a7b9c8d01-2 and more", False),
            ):
                path = Path(directory) / name
                path.write_bytes(body)
                self.assertEqual(boxes.is_a_relay_probe_object(str(path), len(body)), is_probe, name)
                self.assertEqual("written-by=relay-probe" in boxes.entry_of(str(path)), is_probe, name)

    def test_the_probe_bodies_are_the_ones_the_relay_writes(self):
        source = (repository_root / ".dependency" / "buzz-relay" / "src" / "crates" / "buzz-relay" / "src" / "api" / "git" / "store.rs").read_text()
        for phase in ("sequential", "inm-race"):
            self.assertIn(f'format!("probe-{phase}-{{nonce}}-{{round}}")', source)
        self.assertIn('format!("probe-etag-{round}-{nonce}")', source)
        self.assertIn('b"probe-pointer-seed"', source)
        self.assertIn('format!("round={round},racer={i},nonce={nonce}")', source)
        self.assertIn("let nonce = uuid::Uuid::new_v4();", source)

    def test_what_is_exempt_under_the_state_root_is_what_the_backup_and_the_journal_leave_out(self):
        backup = (repository_root / "internal" / "companyhost" / "backup.go").read_text()
        excluded = re.search(r"Role: stateRole, Path: [^,]+, Excluded: \[\]string\{([^}]*)\}", backup).group(1)
        barrier = (repository_root / "internal" / "filestream" / "barrier.go").read_text()
        barrier_directory = re.search(r'BarrierDirectoryName = "([^"]+)"', barrier).group(1)
        package = (repository_root / "internal" / "runtime" / "blueclaw" / "company_host_package.go").read_text()
        box_state = re.search(r'CompanyHostBoxStatePath\s+= "([^"]+)"', package).group(1)
        self.assertIn("filepath.Base(blueclaw.CompanyHostBoxStatePath)", (repository_root / "internal" / "companyhost" / "file_journal_roots.go").read_text())
        left_out = {*re.findall(r'"([^"]+)"', excluded), barrier_directory, box_state.rsplit("/", 1)[-1]}
        exempt = {path for role, path in boxes.EXEMPT if role == "state"}
        self.assertEqual(sorted(exempt - left_out), [])

    def test_a_member_writes_slower_than_the_relay_lets_a_person_write(self):
        source = (repository_root / ".dependency" / "buzz-relay" / "src" / "crates" / "buzz-auth" / "src" / "rate_limit.rs").read_text()
        per_minute = int(re.search(r"fn default_human_msg\(\) -> u64 \{\s*(\d+)\s*\}", source).group(1))
        self.assertLess(60 / boxes.MESSAGE_INTERVAL_SECONDS, per_minute)

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


class ProbeTests(unittest.TestCase):
    def test_a_commit_is_refused_only_when_the_database_answers_so_and_holds_no_probe_row(self):
        self.assertTrue(boxes.fence_refused_the_commit("ERROR:  cannot execute INSERT in a read-only transaction\ninsert=1\nkept=0"))
        self.assertTrue(boxes.fence_refused_the_commit("psql: error: connection refused\ninsert=2\nkept=psql: error"))
        self.assertFalse(boxes.fence_refused_the_commit("insert=0\nkept=1"))
        self.assertFalse(boxes.fence_refused_the_commit("insert=124\nkept=1"))
        self.assertFalse(boxes.fence_refused_the_commit("insert=124\nkept=0"))
        self.assertFalse(boxes.fence_refused_the_commit("probe=the rig remembered no database URL for this box"))

    def test_the_agent_is_at_rest_with_no_turn_running_its_newest_message_taken_and_nothing_queued(self):
        self.assertTrue(boxes.agent_is_at_rest("running=\ntaken=1\ndelivered=0\nqueued=0\nkeys=\n"))
        self.assertTrue(boxes.agent_is_at_rest("running=\ntaken=0\ndelivered=1\nqueued=0\n"))
        self.assertFalse(boxes.agent_is_at_rest("running=running\ntaken=1\ndelivered=1\nqueued=0\n"))
        self.assertFalse(boxes.agent_is_at_rest("running=\ntaken=0\ndelivered=0\nqueued=0\n"))
        self.assertFalse(boxes.agent_is_at_rest("running=\ntaken=1\ndelivered=1\nqueued=1\n"))

    def test_a_schedule_ran_when_a_task_run_names_its_session(self):
        self.assertTrue(boxes.schedule_ran("runs=1 first=2026-11-22 01:00:00+00\nnext_run_at=none failures=0 error="))
        self.assertFalse(boxes.schedule_ran("runs=0 first=none\nnext_run_at=2026-11-22 failures=0 error="))

    def test_the_assertion_is_signed_for_the_audience_and_lifetime_the_box_uses(self):
        source = (repository_root / "internal" / "box" / "assertion.go").read_text()
        self.assertEqual(re.search(r'assertionAudience = "([^"]+)"', source).group(1), boxes.BOX_ASSERTION_AUDIENCE)
        self.assertEqual(re.search(r"assertionLifetime = time\.(\w+)", source).group(1), "Minute")
        self.assertEqual(boxes.BOX_ASSERTION_LIFETIME_SECONDS, 60)

    def test_the_schedule_is_asked_for_where_and_as_whom_capabilityd_asks_for_it(self):
        tool = (repository_root / "internal" / "capabilityd" / "schedule_tool.go").read_text()
        self.assertIn(f'"schedule_create": {{admindPath: "{boxes.SCHEDULE_TOOL_PATH}"', tool)
        socket = (repository_root / "internal" / "capabilityd" / "admind_requester_socket.go").read_text()
        self.assertEqual(re.search(r'admindRequesterEmailHeader\s+= "([^"]+)"', socket).group(1), boxes.REQUESTER_EMAIL_HEADER)

    def test_the_launch_origin_and_the_schedule_session_are_named_as_the_agent_names_them(self):
        agent = repository_root / ".dependency" / "blueclaw" / "internal" / "task"
        names = (agent / "host_task_event_names.go").read_text()
        self.assertEqual(re.search(r'TaskEventLaunchOrigin\s+= "([^"]+)"', names).group(1), boxes.LAUNCH_ORIGIN_EVENT)
        schedule = (agent / "schedule.go").read_text()
        self.assertEqual(re.search(r'scheduleSessionPrefix = "([^"]+)"', schedule).group(1), boxes.SCHEDULE_SESSION_PREFIX)


if __name__ == "__main__":
    unittest.main()
