import hashlib
import importlib.machinery
import importlib.util
import io
import json
import sys
import tarfile
import re
import subprocess
import tempfile
import unittest
import urllib.request
from pathlib import Path

repository_root = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(repository_root / "tools"))

import native_install_rig as rig  # noqa: E402


def load_driver():
    loader = importlib.machinery.SourceFileLoader(
        "native_install_driver", str(repository_root / "tools" / "test-native-install")
    )
    specification = importlib.util.spec_from_loader("native_install_driver", loader)
    module = importlib.util.module_from_spec(specification)
    loader.exec_module(module)
    return module


class StandInPackageTests(unittest.TestCase):
    def build(self, version="1.2.3", revision="abc123"):
        directory = Path(self.enterContext(tempfile.TemporaryDirectory()))
        return rig.build_stand_in_package(directory, version, revision)

    def test_the_package_is_an_ar_archive_dpkg_would_recognise(self):
        package = self.build()
        name, payload = rig.read_archive_member(package, "debian-binary")
        self.assertEqual(name, "debian-binary")
        self.assertEqual(payload, b"2.0\n")
        for member in ("control.tar", "data.tar"):
            found, body = rig.read_archive_member(package, member)
            self.assertTrue(found.startswith(member), member)
            self.assertTrue(body)

    def test_the_control_file_carries_the_fields_the_repository_index_needs(self):
        fields = rig.package_fields(self.build(version="9.9.9"))
        self.assertEqual(fields["Package"], "internkim")
        self.assertEqual(fields["Version"], "9.9.9")
        self.assertEqual(fields["Architecture"], "arm64")
        self.assertIn("ca-certificates", fields["Depends"])

    def test_a_continued_description_keeps_the_indentation_a_paragraph_needs(self):
        fields = rig.package_fields(self.build())
        continuation = fields["Description"].splitlines()[1:]
        self.assertTrue(continuation)
        for line in continuation:
            self.assertTrue(line.startswith(" "), repr(line))

    def test_the_data_archive_carries_the_directories_dpkg_unpacks_into(self):
        _, body = rig.read_archive_member(self.build(), "data.tar")
        with tarfile.open(fileobj=io.BytesIO(body), mode="r:*") as archive:
            names = {member.name for member in archive.getmembers()}
            directories = {member.name for member in archive.getmembers() if member.isdir()}
        self.assertIn("./usr/lib/internkim/blueclaw-posix-helper", names)
        self.assertIn("./usr/lib/internkim", directories)

    def test_the_helper_keeps_the_setuid_bit_the_permission_boundary_rests_on(self):
        _, body = rig.read_archive_member(self.build(), "data.tar")
        with tarfile.open(fileobj=io.BytesIO(body), mode="r:*") as archive:
            helper = archive.getmember("./usr/lib/internkim/blueclaw-posix-helper")
        self.assertEqual(helper.mode, 0o4755)

    def test_two_versions_differ_in_the_revision_their_services_report(self):
        first = self.build(version="1.0.0", revision="oneoneone")
        second = self.build(version="1.0.1", revision="twotwotwo")
        self.assertNotEqual(rig.package_fields(first)["Version"], rig.package_fields(second)["Version"])
        _, body = rig.read_archive_member(second, "data.tar")
        with tarfile.open(fileobj=io.BytesIO(body), mode="r:*") as archive:
            server = archive.extractfile("./usr/lib/internkim/health-server").read().decode()
        self.assertIn("twotwotwo", server)
        self.assertNotIn("@REVISION@", server)


class ReleaseTests(unittest.TestCase):
    def published(self):
        directory = Path(self.enterContext(tempfile.TemporaryDirectory()))
        package = rig.build_stand_in_package(directory / "packages", "1.0.0", "one")
        built = rig.write_release_directory(directory / "built", package)
        release = rig.Release(directory / "served")
        release.publish(built)
        release.serve()
        self.addCleanup(release.stop)
        return release, built

    def fetch(self, release, name):
        with urllib.request.urlopen(f"{release.download_url('127.0.0.1')}/{name}") as answered:
            return answered.read()

    def test_the_package_and_its_checksum_are_served_where_github_serves_the_latest_release(self):
        release, built = self.published()
        name = rig.asset_name(".deb")
        package = self.fetch(release, name)
        self.assertEqual(package, (built / name).read_bytes())
        listed = self.fetch(release, rig.CHECKSUMS_NAME).decode()
        self.assertEqual(listed, f"{hashlib.sha256(package).hexdigest()}  {name}\n")
        self.assertIn(f"/{rig.RELEASE_DOWNLOAD_PATH}", release.download_url("127.0.0.1"))

    def test_the_install_script_is_served_beside_the_release(self):
        release, _ = self.published()
        with urllib.request.urlopen(f"http://127.0.0.1:{release.port}/install.sh") as answered:
            self.assertEqual(answered.read(), rig.INSTALL_SCRIPT_PATH.read_bytes())

    def test_a_replaced_file_is_served_and_putting_it_back_leaves_the_build_untouched(self):
        release, built = self.published()
        name = rig.asset_name(".deb")
        original = (built / name).read_bytes()
        restore = release.replace(name, b"tampered")
        self.assertEqual(self.fetch(release, name), b"tampered")
        self.assertEqual((built / name).read_bytes(), original)
        restore()
        self.assertEqual(self.fetch(release, name), original)

    def test_the_rig_asks_for_the_names_the_build_writes(self):
        source = (rig.REPOSITORY_ROOT / "internal" / "cli" / "release_package.go").read_text()
        suffixes = re.findall(r'^\t\tSuffix:\s+"([^"]+)"', source, re.MULTILINE)
        self.assertEqual(suffixes, [".deb", ".rpm", ".pkg.tar.zst"])
        builder = re.search(
            r'return blueclaw\.CompanyPackageName \+ "-" \+ architecture \+ format\.Suffix', source)
        self.assertIsNotNone(builder, "release_package.go no longer names an asset <package>-<architecture><suffix>")
        self.assertEqual(rig.asset_name(".rpm"), f"{rig.PACKAGE_NAME}-{rig.ARCHITECTURE}.rpm")


class MergedUsrTests(unittest.TestCase):
    def setUp(self):
        self.driver = load_driver()

    def test_a_unit_path_is_asked_of_dpkg_under_both_of_its_names(self):
        spellings = self.driver.alternate_spellings("/usr/lib/systemd/system/internkim-admind.service")
        self.assertEqual(
            spellings,
            {
                "/usr/lib/systemd/system/internkim-admind.service",
                "/lib/systemd/system/internkim-admind.service",
            },
        )

    def test_a_path_outside_the_merged_directories_keeps_its_one_name(self):
        self.assertEqual(self.driver.alternate_spellings("/etc/internkim/runtime.json"), {"/etc/internkim/runtime.json"})


class DependencyReadingTests(unittest.TestCase):
    def test_every_name_in_the_packages_own_depends_line_is_read_back(self):
        directory = Path(self.enterContext(tempfile.TemporaryDirectory()))
        package = rig.build_stand_in_package(directory, "1.0.0", "one")
        self.assertEqual(
            rig.dependency_names(rig.package_fields(package)["Depends"]), ["ca-certificates"]
        )

    def test_a_name_bounded_from_both_sides_is_asked_for_once(self):
        self.assertEqual(
            rig.dependency_names("postgresql (>= 14), postgresql (<< 18), ca-certificates"),
            ["postgresql", "ca-certificates"],
        )

    def test_every_alternative_of_a_clause_is_kept_in_order(self):
        self.assertEqual(
            rig.dependency_alternatives("postgresql, redis-server | valkey-server, jq (>= 1.6)"),
            [["postgresql"], ["redis-server", "valkey-server"], ["jq"]],
        )

    def test_the_first_alternative_with_a_candidate_is_chosen(self):
        policy = {"redis-server": "(none)", "valkey-server": "8.1.1-1", "jq": "1.7"}
        script = "set -o pipefail\napt-cache() { case \"$2\" in " + " ".join(
            f"{name}) echo '  Candidate: {candidate}';;" for name, candidate in policy.items()
        ) + " esac; }\n" + rig.CANDIDATE_CHOICE_COMMAND % "'redis-server|valkey-server' 'jq' 'unknown-a|unknown-b'"
        chosen = subprocess.run(["bash", "-c", script], capture_output=True, text=True).stdout.split()
        self.assertEqual(chosen, ["valkey-server", "jq", "unknown-a"])

    def test_a_versioned_or_alternative_dependency_reduces_to_a_name_apt_can_install(self):
        self.assertEqual(
            rig.dependency_names("postgresql (>= 14), chromium | chromium-browser, jq"),
            ["postgresql", "chromium", "jq"],
        )

    def test_the_plans_own_dependency_line_survives_the_wrapping_it_is_written_with(self):
        self.assertEqual(
            rig.dependency_names("ca-certificates, curl, git,\n openssl, python3-venv"),
            ["ca-certificates", "curl", "git", "openssl", "python3-venv"],
        )


if __name__ == "__main__":
    unittest.main()


class AddressesTheRigSpellsTests(unittest.TestCase):
    """Every port and path the rig asks at belongs to `internal/runtime/blueclaw`.

    A Go constant and a Python constant that mean the same thing are two copies,
    and the second one is how the rig came to poll 8081 for a relay that answers
    on 3000. These read the canonical ones and fail when they drift.
    """

    def declared_in_go(self, file_name, name):
        source = (rig.REPOSITORY_ROOT / "internal" / "runtime" / "blueclaw" / file_name).read_text()
        match = re.search(rf'^\t{name}\s+=\s*"([^"]+)"', source, re.MULTILINE)
        self.assertIsNotNone(match, f"internal/runtime/blueclaw no longer declares {name}")
        return match.group(1)

    def contract(self, name):
        return self.declared_in_go("blueclaw_contract.go", name)

    def package(self, name):
        return self.declared_in_go("company_host_package.go", name)

    def test_the_messenger_is_asked_where_its_unit_makes_it_answer(self):
        port, path = rig.MESSENGER_READINESS
        self.assertEqual(f"127.0.0.1:{port}", self.contract("BuzzRelayBindAddress"))
        self.assertEqual(path, self.contract("BuzzRelayReadinessPath"))

    def test_the_agent_is_asked_where_its_runtime_makes_it_listen(self):
        port, path = rig.AGENT_HEALTH
        self.assertEqual(f"http://127.0.0.1:{port}", self.contract("BlueclawBaseURL"))
        self.assertEqual(path, self.contract("BlueclawHealthCheckPath"))

    def test_the_revision_is_read_from_the_one_endpoint_that_carries_it(self):
        port, path = rig.REVISION_PROBE
        self.assertEqual(f"127.0.0.1:{port}", self.package("CompanyHostAdmindListenAddress"))
        self.assertEqual(path, self.contract("BlueclawHealthCheckPath"))
        blueclaw_health = (
            rig.REPOSITORY_ROOT / ".dependency" / "blueclaw" / "internal" / "httpserver" / "health_handler.go"
        )
        if blueclaw_health.exists():
            self.assertNotIn(
                "gitRevision",
                blueclaw_health.read_text(),
                "blueclaw's health now carries a revision; the rig can read the agent's own",
            )

    def test_the_conffile_and_the_condition_are_the_paths_the_package_uses(self):
        self.assertEqual(rig.CONFFILE_PATH, self.package("CompanyHostSettingsPath"))
        self.assertEqual(rig.COMPANY_CONDITION_PATH, self.package("CompanyHostEnvironmentPath"))

    def test_the_skills_tree_is_where_the_package_delivers_the_skills(self):
        self.assertEqual(rig.SKILLS_PATH, self.package("CompanyPackageSkillsPath"))

    def test_the_messenger_store_is_read_through_the_file_the_install_writes(self):
        self.assertEqual(rig.MESSENGER_DATABASE_PATH, self.package("CompanyHostBuzzDatabasePath"))

    def test_the_bridge_is_asked_where_its_unit_makes_it_answer(self):
        port, path = rig.MESSENGER_BRIDGE
        self.assertEqual(f"http://127.0.0.1:{port}", self.package("CompanyHostChatdEndpoint"))
        self.assertEqual(path, self.contract("ChatdHealthPath"))


class TheSigningKeyIsTheOneTheStackPublishes(unittest.TestCase):
    """The app's key has to be the private half of what every verifier holds.

    A key derived from the shared secret satisfies nothing that reads a key set,
    which is what kept the host's handshake from opening and step 5 from
    carrying a message. Skipped when no stack is up; there is no local key to
    read without one.
    """

    def signing_key(self):
        try:
            return json.loads(rig.local_plane_signing_key())
        except rig.RigFailure as refusal:
            self.skipTest(str(refusal))

    def test_the_definition_emits_a_private_key_a_token_can_be_signed_with(self):
        key = self.signing_key()
        self.assertEqual(key["kty"], "EC")
        self.assertEqual(key["alg"], "ES256")
        self.assertIn("d", key)

    def local_plane_address(self):
        try:
            return rig.local_plane_settings()["API_URL"]
        except rig.RigFailure as refusal:
            self.skipTest(str(refusal))

    def test_that_key_is_the_one_the_record_publishes(self):
        key = self.signing_key()
        address = self.local_plane_address() + "/auth/v1/.well-known/jwks.json"
        with urllib.request.urlopen(address, timeout=10) as answered:
            published = json.loads(answered.read())
        self.assertIn(key["kid"], [held["kid"] for held in published["keys"]])


class StandInDeclinesTests(unittest.TestCase):
    """The stand-in has to decline to start for the same reason the package does."""

    def units(self):
        directory = Path(self.enterContext(tempfile.TemporaryDirectory()))
        package = rig.build_stand_in_package(directory, "1.0.0", "one")
        _, body = rig.read_archive_member(package, "data.tar")
        with tarfile.open(fileobj=io.BytesIO(body), mode="r:*") as archive:
            return {
                member.name: archive.extractfile(member).read().decode()
                for member in archive.getmembers()
                if member.name.endswith(".service")
            }

    def test_every_stand_in_unit_waits_on_the_file_an_install_writes(self):
        units = self.units()
        self.assertTrue(units)
        for name, contents in units.items():
            self.assertIn(f"ConditionPathExists={rig.COMPANY_CONDITION_PATH}", contents, name)

    def test_the_stand_in_admin_gateway_answers_every_probe_on_its_port(self):
        admind = self.units()["./lib/systemd/system/internkim-admind.service"]
        served = next(
            line.split()[1:] for line in admind.splitlines() if line.startswith("ExecStart=")
        )
        port, _ = rig.ADMIN_GATEWAY_HEALTH
        for probe_port, path in rig.READINESS_PROBES:
            if probe_port == port:
                self.assertIn(path, served[1:], f"the stand-in admind does not answer {path}")

    def test_the_stand_in_ships_the_conffile_the_package_ships(self):
        directory = Path(self.enterContext(tempfile.TemporaryDirectory()))
        package = rig.build_stand_in_package(directory, "1.0.0", "one")
        _, control = rig.read_archive_member(package, "control.tar")
        with tarfile.open(fileobj=io.BytesIO(control), mode="r:*") as archive:
            conffiles = archive.extractfile("./conffiles").read().decode()
        self.assertEqual(conffiles.strip(), rig.CONFFILE_PATH)


class WhatThePackageCarriesHasOneSpelling(unittest.TestCase):
    """The guest is asked about paths and imports that belong to `internal/runtime/blueclaw`."""

    def blueclaw_source(self, file_name):
        return (rig.REPOSITORY_ROOT / "internal" / "runtime" / "blueclaw" / file_name).read_text()

    def declared(self, file_name, name):
        match = re.search(rf'^\s*(?:const\s+)?{name}\s+=\s*"([^"]+)"', self.blueclaw_source(file_name), re.MULTILINE)
        self.assertIsNotNone(match, f"internal/runtime/blueclaw no longer declares {name}")
        return match.group(1)

    def test_the_modules_the_rig_imports_are_the_ones_the_build_checks(self):
        source = self.blueclaw_source("host_python.go")
        declared = re.search(r"const documentModulesTheConversionImports = ((?:\"[^\"]*\"\s*\+?\s*)+)", source)
        self.assertIsNotNone(declared, "documentModulesTheConversionImports is no longer a string literal")
        joined = "".join(re.findall(r'"([^"]*)"', declared.group(1)))
        self.assertEqual(rig.DOCUMENT_MODULES_THE_CONVERSION_IMPORTS, joined)

    def test_the_paths_the_rig_reads_are_the_ones_the_package_installs(self):
        driver = load_driver()
        self.assertEqual(rig.HOST_PYTHON_VERSION, self.declared("host_python.go", "HostPythonVersion"))
        library = self.declared("company_host_package.go", "CompanyPackageLibraryRoot")
        layout = self.blueclaw_source("company_host_layout.go")
        for path in rig.DOCUMENT_ENVIRONMENT_PATHS:
            self.assertTrue(path.startswith(library + "/"), path)
            self.assertIn(f'layout.LibraryRoot + "{path[len(library):]}"', layout)
        self.assertIn('return layout.PythonRoot() + "/bin"', layout)
        self.assertIn('return layout.PythonCommandsPath() + "/python3"', layout)
        self.assertEqual(rig.HOST_PYTHON_PATH, rig.DOCUMENT_ENVIRONMENT_PATHS[0] + "/bin/python3")
        self.assertEqual(driver.CARRIED_FONT_PATH, self.declared("company_host_package.go", "CompanyPackageDocumentFontPath"))

    def test_the_backup_the_rig_reads_is_the_one_the_package_schedules(self):
        self.assertEqual(rig.BACKUP_SERVICE_NAME, self.declared("company_host_package.go", "CompanyHostBackupServiceName"))
        self.assertEqual(rig.BACKUPS_DIRECTORY, self.declared("company_host_package.go", "CompanyHostBackupsPath"))
        self.assertEqual(rig.DATABASE_USER, self.declared("company_host_data_services.go", "CompanyHostDatabaseUser"))
        self.assertEqual(rig.expected_enablement(rig.BACKUP_UNIT_NAME), "static")
        self.assertEqual(rig.expected_enablement(rig.BACKUP_TIMER_NAME), "enabled")

    def test_every_distribution_the_rig_boots_is_one_the_package_is_promised_to_install_on(self):
        self.assertEqual(sorted(rig.DISTRIBUTIONS), ["debian-13", "ubuntu-22.04", "ubuntu-24.04"])

    def test_the_unit_the_rig_expects_to_be_running_is_the_box(self):
        driver = load_driver()
        self.assertEqual(
            driver.BOX_UNIT_NAME, self.declared("company_host_package.go", "BoxServiceName") + ".service"
        )


import shlex  # noqa: E402

import native_install_agent_update as agent_update  # noqa: E402


class AgentUpdateRigTests(unittest.TestCase):
    def test_the_longest_outage_is_measured_from_the_first_silent_reading_to_the_next_answer(self):
        log = "100.0 up\n101.0 down\n102.0 down\n104.5 up\n105.0 down\n106.0 up\n"
        self.assertEqual(agent_update.downtime_from(log), (3.5, True, 6))

    def test_an_outage_still_open_is_not_reported_as_back(self):
        self.assertEqual(agent_update.downtime_from("1.0 up\n2.0 down\n")[1], False)

    def test_the_promised_downtime_is_the_one_admind_states(self):
        source = (repository_root / "internal" / "admind" / "host_update.go").read_text()
        self.assertIn(f"expectedHostUpdateDowntimeSeconds = {agent_update.promised_downtime_seconds()}", " ".join(source.split()))

    def test_the_paths_the_rig_reads_are_the_ones_the_host_writes(self):
        hostupdate = repository_root / "internal" / "hostupdate"
        self.assertIn('"host-update.json"', (hostupdate / "note.go").read_text())
        self.assertIn('"release-channel"', (hostupdate / "machine.go").read_text())
        self.assertEqual(agent_update.UPDATE_UNIT_NAME, "internkim-host-update.service")

    def test_a_curl_answer_splits_into_its_status_and_document(self):
        self.assertEqual(agent_update.answer_of('{"errorCode":"update_in_progress"}\nstatus=409\n'), ("409", {"errorCode": "update_in_progress"}))
        self.assertEqual(agent_update.answer_of("not json\nstatus=502"), ("502", {}))

    def test_the_admind_call_carries_the_requester_and_the_body_whole(self):
        arguments = shlex.split(agent_update.ask_admind_command("/host/api/update", "member1@example.com", {"input": {}, "note": "it's"}))
        self.assertIn("X-INTERNKIM-REQUESTER-EMAIL: member1@example.com", arguments)
        self.assertEqual(json.loads(arguments[arguments.index("--data") + 1]), {"input": {}, "note": "it's"})
        self.assertEqual(arguments[-1], "http://internkim/host/api/update")

    def test_the_stable_listing_answers_the_release_list(self):
        with tempfile.TemporaryDirectory() as directory:
            agent_update.write_stable_listing(directory, ["v2", "v1"], "2026-10-02T00:00:00Z")
            releases = Path(directory) / "api" / "releases"
            self.assertEqual([release["tag_name"] for release in json.loads((releases / "index.html").read_text())], ["v2", "v1"])

    def test_the_update_units_drop_in_is_named_for_the_unit_the_gateway_starts(self):
        unit = (repository_root / "internal" / "hostupdate" / "unit.go").read_text()
        self.assertIn(f'UnitName = "{agent_update.UPDATE_UNIT_NAME.removesuffix(".service")}"', unit)
        self.assertIn(f"/{agent_update.UPDATE_UNIT_NAME}.d/", agent_update.UPDATE_UNIT_DROP_IN_PATH)


import native_install_messenger as messenger  # noqa: E402


class MessengerRigTests(unittest.TestCase):
    def declared_in_package(self, name):
        source = (repository_root / "internal" / "runtime" / "blueclaw" / "company_host_package.go").read_text()
        match = re.search(rf'^\t{name}\s+=\s*"([^"]+)"', source, re.MULTILINE)
        self.assertIsNotNone(match, f"company_host_package.go no longer declares {name}")
        return match.group(1)

    def test_the_guest_is_asked_at_the_addresses_and_paths_the_package_declares(self):
        self.assertEqual(messenger.CHATD_ENDPOINT, self.declared_in_package("CompanyHostChatdEndpoint"))
        self.assertEqual(messenger.IDENTITY_SEED_PATH, self.declared_in_package("CompanyHostIdentitySeedPath"))
        self.assertEqual(messenger.AGENT_DATABASE_PATH, rig.COMPANY_CONDITION_PATH)
        self.assertEqual(messenger.ADMIND_SOCKET_PATH, agent_update.ADMIND_SOCKET_PATH)
        identity = (repository_root / "internal" / "buzzidentity" / "identity.go").read_text()
        self.assertIn(f'AgentSubject = "{messenger.AGENT_IDENTITY_NAME}"', identity)
        self.assertEqual(messenger.AGENT_UNIT_NAME, load_driver().AGENT_UNIT_NAME)

    def test_a_persons_key_is_their_lowercased_address_under_the_seed(self):
        expected = hashlib.sha256(b"seed|secret|member1@example.com").hexdigest()
        self.assertEqual(messenger.buzz_secret("seed", "  Member1@Example.com "), expected)

    def test_the_policy_path_is_the_one_the_agents_unit_passes(self):
        shown = "{ path=/usr/bin/blueclaw ; argv[]=/usr/bin/blueclaw -listen 127.0.0.1:8080 -policy /run/internkim/policy.json -x y ; ignore_errors=no }"
        self.assertEqual(messenger.policy_path_of(shown), "/run/internkim/policy.json")
        self.assertEqual(messenger.policy_path_of("argv[]=/usr/bin/blueclaw"), "")

    def test_a_person_without_an_address_is_not_a_person_the_messenger_can_reach(self):
        policy = {"people": [{"personID": "a", "emails": ["a@example.com", "b@example.com"]}, {"personID": "c", "emails": []}, {"personID": "d"}]}
        self.assertEqual(messenger.people_of(policy), [{"personID": "a", "email": "a@example.com"}])

    def test_a_marker_is_found_wherever_a_message_keeps_its_text(self):
        document = {"messages": [{"id": "1", "content": {"body": "hello"}}, {"id": "2", "parts": ["x", "rig marker 42"]}]}
        self.assertTrue(messenger.holds_text(document, "marker 42"))
        self.assertFalse(messenger.holds_text(document, "marker 43"))

    def test_only_a_read_that_failed_is_a_failed_read(self):
        bodies = [
            json.dumps({"tool": "read", "failure": {"code": "not_found"}}),
            json.dumps({"tool": "read", "failure": {"code": "invalid_input"}}),
            json.dumps({"tool": "read", "output": {"content": "ok"}}),
            json.dumps({"tool": "write", "failure": {"code": "not_found"}}),
            "not json",
        ]
        self.assertEqual(len(messenger.reads_that_failed(bodies)), 2)

    def test_a_message_id_is_checked_before_it_reaches_a_query(self):
        self.assertTrue(messenger.is_a_hex_identifier("0123abcdef0123abcdef"))
        self.assertFalse(messenger.is_a_hex_identifier("1'; drop table task_run; --"))
        self.assertFalse(messenger.is_a_hex_identifier(""))

    def test_the_picture_is_a_png_and_the_word_is_not_in_what_the_agent_is_asked(self):
        picture = Path(load_driver().MESSENGER_PICTURE_SOURCE)
        self.assertTrue(picture.read_bytes().startswith(b"\x89PNG\r\n\x1a\n"))
        self.assertNotIn(messenger.WORD_THE_PICTURE_CARRIES.lower(), messenger.PICTURE_QUESTION.lower())

    def test_every_action_the_rig_asks_the_guest_for_exists(self):
        driver_source = (repository_root / "tools" / "test-native-install").read_text()
        asked = set(re.findall(r'ask_the_messenger\(\s*"(\w+)"', driver_source)) | set(re.findall(r'wait_for_the_messenger\(\s*"(\w+)"', driver_source))
        self.assertTrue(asked)
        self.assertLessEqual(asked, set(messenger.ACTIONS))

    def test_the_picture_is_served_whole_for_chatd_to_fetch(self):
        server = messenger.serving_once(b"\x89PNG-bytes", "image/png")
        try:
            with urllib.request.urlopen(f"http://127.0.0.1:{server.server_port}/picture", timeout=5) as response:
                self.assertEqual(response.read(), b"\x89PNG-bytes")
                self.assertEqual(response.headers["Content-Type"], "image/png")
        finally:
            server.shutdown()
