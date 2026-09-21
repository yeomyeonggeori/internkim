import gzip
import importlib.machinery
import importlib.util
import io
import subprocess
import sys
import tarfile
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
            release.repository_directory / "dists" / "stable" / "main" / "binary-arm64" / "Packages"
        ).read_text()
        self.assertIn("Version: 1.0.0", index)
        self.assertIn("Version: 1.0.1", index)
        self.assertEqual(index.count("SHA256: "), 2)
        self.assertEqual(index.count("Filename: pool/main/i/internkim/"), 2)

    def test_the_compressed_index_holds_the_same_bytes(self):
        release, directory = self.repository()
        release.publish(rig.build_stand_in_package(directory / "packages", "1.0.0", "one"))
        binary = release.repository_directory / "dists" / "stable" / "main" / "binary-arm64"
        self.assertEqual(gzip.decompress((binary / "Packages.gz").read_bytes()), (binary / "Packages").read_bytes())

    def test_the_release_file_is_signed_and_checksums_the_indices(self):
        release, directory = self.repository()
        release.publish(rig.build_stand_in_package(directory / "packages", "1.0.0", "one"))
        suite = release.repository_directory / "dists" / "stable"
        signed = (suite / "InRelease").read_text()
        self.assertIn("BEGIN PGP SIGNED MESSAGE", signed)
        self.assertIn("Suite: stable", signed)
        for name in ("main/binary-arm64/Packages", "main/binary-arm64/Packages.gz"):
            self.assertIn(name, (suite / "Release").read_text())

    def test_the_signature_verifies_against_the_exported_keyring(self):
        release, directory = self.repository()
        release.publish(rig.build_stand_in_package(directory / "packages", "1.0.0", "one"))
        verified = subprocess.run(
            [
                "gpg", "--homedir", str(release.keyring_directory), "--batch", "--verify",
                str(release.repository_directory / "dists" / "stable" / "InRelease"),
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
        address = f"http://127.0.0.1:{port}/deb/dists/stable/InRelease"
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
