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
            rig.dependency_alternatives("postgresql, postgresql-18-pgvector | postgresql-17-pgvector, jq (>= 1.6)"),
            [["postgresql"], ["postgresql-18-pgvector", "postgresql-17-pgvector"], ["jq"]],
        )

    def test_the_first_alternative_with_a_candidate_is_chosen(self):
        policy = {"postgresql-18-pgvector": "(none)", "postgresql-17-pgvector": "0.8.0-1", "jq": "1.7"}
        script = "apt-cache() { case \"$2\" in " + " ".join(
            f"{name}) echo '  Candidate: {candidate}';;" for name, candidate in policy.items()
        ) + " esac; }\n" + rig.CANDIDATE_CHOICE_COMMAND % "'postgresql-18-pgvector|postgresql-17-pgvector' 'jq' 'unknown-a|unknown-b'"
        chosen = subprocess.run(["bash", "-c", script], capture_output=True, text=True).stdout.split()
        self.assertEqual(chosen, ["postgresql-17-pgvector", "jq", "unknown-a"])

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
