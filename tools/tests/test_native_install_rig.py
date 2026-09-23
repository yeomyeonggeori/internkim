import gzip
import importlib.machinery
import importlib.util
import io
import json
import subprocess
import sys
import tarfile
import re
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
        self.assertIn("python3", fields["Depends"])

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


class RepositoryTests(unittest.TestCase):
    def repository(self):
        directory = Path(self.enterContext(tempfile.TemporaryDirectory()))
        release = rig.Release(directory / "release")
        release.directory.mkdir(parents=True)
        self.addCleanup(release.stop)
        release.generate_signing_key()
        return release, directory

    def test_the_index_names_every_published_package_with_its_checksum(self):
        release, directory = self.repository()
        for version in ("1.0.0", "1.0.1"):
            release.publish(rig.build_stand_in_package(directory / "packages", version, version))
        index = (
            release.repository_directory / "dists" / rig.SUITE / "main" / "binary-arm64" / "Packages"
        ).read_text()
        self.assertIn("Version: 1.0.0", index)
        self.assertIn("Version: 1.0.1", index)
        self.assertEqual(index.count("SHA256: "), 2)
        self.assertEqual(index.count("Filename: pool/main/i/internkim/"), 2)

    def test_the_compressed_index_holds_the_same_bytes(self):
        release, directory = self.repository()
        release.publish(rig.build_stand_in_package(directory / "packages", "1.0.0", "one"))
        binary = release.repository_directory / "dists" / rig.SUITE / "main" / "binary-arm64"
        self.assertEqual(gzip.decompress((binary / "Packages.gz").read_bytes()), (binary / "Packages").read_bytes())

    def test_the_release_file_is_signed_and_checksums_the_indices(self):
        release, directory = self.repository()
        release.publish(rig.build_stand_in_package(directory / "packages", "1.0.0", "one"))
        suite = release.repository_directory / "dists" / rig.SUITE
        signed = (suite / "InRelease").read_text()
        self.assertIn("BEGIN PGP SIGNED MESSAGE", signed)
        self.assertIn(f"Suite: {rig.SUITE}", signed)
        for name in ("main/binary-arm64/Packages", "main/binary-arm64/Packages.gz"):
            self.assertIn(name, (suite / "Release").read_text())

    def test_the_signature_verifies_against_the_exported_keyring(self):
        release, directory = self.repository()
        release.publish(rig.build_stand_in_package(directory / "packages", "1.0.0", "one"))
        verified = subprocess.run(
            [
                "gpg", "--homedir", str(release.keyring_directory), "--batch", "--verify",
                str(release.repository_directory / "dists" / rig.SUITE / "InRelease"),
            ],
            capture_output=True,
            text=True,
        )
        self.assertEqual(verified.returncode, 0, verified.stderr)
        self.assertTrue(release.public_keyring_path.read_bytes())

    def test_the_server_answers_a_conditional_request_with_the_body(self):
        release, directory = self.repository()
        release.publish(rig.build_stand_in_package(directory / "packages", "1.0.0", "one"))
        port = release.serve()
        address = f"http://127.0.0.1:{port}/deb/dists/{rig.SUITE}/InRelease"
        first = urllib.request.urlopen(address)
        request = urllib.request.Request(address, headers={"If-Modified-Since": first.headers["Last-Modified"]})
        self.assertEqual(urllib.request.urlopen(request).status, 200)


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
            rig.dependency_names(rig.package_fields(package)["Depends"]), ["python3", "ca-certificates"]
        )

    def test_a_name_bounded_from_both_sides_is_asked_for_once(self):
        self.assertEqual(
            rig.dependency_names("python3 (>= 3.13), python3 (<< 3.14), ca-certificates"),
            ["python3", "ca-certificates"],
        )

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


class RepositoryShapeTests(unittest.TestCase):
    """The repository's shape belongs to `internal/aptrepository`.

    The rig spells two of its names in a URL. A Go constant and a Python
    constant that mean the same thing are two copies, so this reads the
    canonical one and fails when they drift.
    """

    def declared_in_go(self, name):
        source = (rig.REPOSITORY_ROOT / "internal" / "aptrepository" / "repository.go").read_text()
        match = re.search(rf'^\t{name}\s*=\s*"([^"]+)"', source, re.MULTILINE)
        self.assertIsNotNone(match, f"internal/aptrepository no longer declares {name}")
        return match.group(1)

    def test_the_prefix_the_rig_serves_from_is_the_one_the_builder_publishes_to(self):
        self.assertEqual(rig.REPOSITORY_PREFIX, self.declared_in_go("Prefix"))

    def test_the_keyring_the_rig_installs_is_the_one_the_builder_exports(self):
        self.assertEqual(rig.KEYRING_NAME, self.declared_in_go("KeyringName"))

    def test_the_debian_release_the_rig_runs_is_the_one_the_suite_name_carries(self):
        self.assertEqual(rig.DEBIAN_SUITE, self.declared_in_go("DebianSuite"))

    def test_the_suite_the_rig_asks_apt_for_is_one_the_builder_publishes(self):
        source = (rig.REPOSITORY_ROOT / "internal" / "aptrepository" / "repository.go").read_text()
        declared = re.search(r"^var DefaultSuite = (.+)$", source, re.MULTILINE)
        self.assertIsNotNone(declared, "internal/aptrepository no longer declares DefaultSuite")
        self.assertEqual(declared.group(1).strip(), 'DebianSuite + "-stable"')
        self.assertEqual(rig.SUITE, rig.DEBIAN_SUITE + "-stable")

    def test_the_guest_installs_that_keyring_where_the_source_looks_for_it(self):
        self.assertTrue(rig.KEYRING_PATH.endswith("/" + rig.KEYRING_NAME))


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

    def test_that_key_is_the_one_the_record_publishes(self):
        key = self.signing_key()
        address = rig.local_plane_settings()["API_URL"] + "/auth/v1/.well-known/jwks.json"
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


class TheSigningKeyVariableHasOneSpelling(unittest.TestCase):
    """The rig names the vault variable in Python and the CLI names it in Go.

    Two hand-kept copies of one name is the defect this repository keeps
    finding, so the Go constant is the canonical one and this reads it.
    """

    def test_the_rig_names_the_same_signing_key_variable(self):
        source = (rig.REPOSITORY_ROOT / "internal" / "aptrepository" / "signing.go").read_text()
        declared = re.search(r'SigningKeyVariable\s*=\s*"([^"]+)"', source)
        self.assertIsNotNone(declared, "aptrepository.SigningKeyVariable is not declared as a literal")
        self.assertEqual(declared.group(1), rig.SIGNING_KEY_VARIABLE)

    def test_no_flag_offers_the_signing_key_a_second_home(self):
        source = (rig.REPOSITORY_ROOT / "internal" / "cli" / "release_apt.go").read_text()
        self.assertNotIn("--signing-key", source)

    def test_the_profile_the_rig_runs_under_leaves_its_signing_key_alone(self):
        manifest = (rig.REPOSITORY_ROOT / ".monkeys").read_text().splitlines()
        profiles = [line.strip()[1:].split(",") for line in manifest if line.strip().startswith("@")]
        self.assertTrue(profiles, ".monkeys declares no profile")
        default_profile = profiles[0][0].strip()
        declared, is_open = [], False
        for line in (line.strip() for line in manifest):
            if line.startswith("@"):
                is_open = default_profile in [name.strip() for name in line[1:].split(",")]
            elif is_open and line and "=" not in line and not line.startswith(("#", "+")):
                declared.append(line)
        self.assertNotIn(rig.SIGNING_KEY_VARIABLE, declared,
                         f"@{default_profile} would hand the rig's CLI the vault's signing key over its throwaway one")
