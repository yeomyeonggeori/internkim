import importlib.machinery
import importlib.util
import re
import unittest
from pathlib import Path

repository_root = Path(__file__).resolve().parents[2]
blueclaw_source_root = repository_root / "internal" / "runtime" / "blueclaw"


def load_rig():
    loader = importlib.machinery.SourceFileLoader("macos_install_rig", str(repository_root / "tools" / "test-macos-install"))
    specification = importlib.util.spec_from_loader("macos_install_rig", loader)
    module = importlib.util.module_from_spec(specification)
    loader.exec_module(module)
    return module


rig = load_rig()


def declared(file_name, name):
    source = (blueclaw_source_root / file_name).read_text()
    match = re.search(rf'^\s*(?:const\s+)?{name}\s+=\s*"([^"]+)"', source, re.MULTILINE)
    if match is None:
        raise AssertionError(f"internal/runtime/blueclaw no longer declares {name} in {file_name}")
    return match.group(1)


class WhatTheRigUndoesIsWhatTheInstallMakes(unittest.TestCase):
    """The rig is Python and the install is Go, so the names it takes back out are
    read from the Go declarations here rather than trusted."""

    def test_the_accounts_are_the_ones_the_install_creates(self):
        accounts = {
            declared("blueclaw_contract.go", "BlueclawUser"),
            declared("blueclaw_contract.go", "RelayUserName"),
            declared("company_host_data_services.go", "CompanyHostDatabaseUser"),
            declared("company_host_data_services.go", "CompanyHostCacheUser"),
        }
        self.assertEqual(set(rig.SERVICE_ACCOUNTS), accounts)

    def test_the_directories_are_the_ones_a_mac_host_makes(self):
        directories = {
            "/var/lib/" + declared("company_host_data_services.go", "CompanyHostDatabaseStateDirectoryName"),
            "/var/lib/" + declared("company_host_data_services.go", "CompanyHostCacheStateDirectoryName"),
            declared("company_host_layout.go", "macBlueclawHomePath"),
            declared("company_host_layout.go", "macCompanyHostWorkspacePath"),
            declared("company_host_package.go", "CompanyHostBrowserStatePath"),
            declared("company_host_package.go", "CompanyHostLogPath"),
            declared("company_host_layout.go", "macCompanyHostRunPath"),
        }
        self.assertEqual({str(path) for path in rig.DATA_DIRECTORIES}, directories)

    def test_the_company_keys_are_never_among_what_undo_removes(self):
        state_root = declared("company_host_package.go", "CompanyHostStateRoot")
        self.assertEqual(str(rig.STATE_ROOT), state_root)
        self.assertNotIn(Path(state_root), rig.DATA_DIRECTORIES)

    def test_the_daemons_are_found_by_the_label_prefix_the_install_writes(self):
        self.assertEqual(rig.LAUNCH_DAEMON_PREFIX, declared("company_host_launchd.go", "CompanyHostLaunchDaemonLabelPrefix"))


if __name__ == "__main__":
    unittest.main()
