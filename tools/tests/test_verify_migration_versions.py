import importlib.machinery
import importlib.util
import pathlib
import unittest


REPOSITORY_ROOT = pathlib.Path(__file__).resolve().parents[2]


def versions_gate_module():
    script = REPOSITORY_ROOT / "tools" / "verify-migration-versions"
    loader = importlib.machinery.SourceFileLoader("verify_migration_versions", str(script))
    specification = importlib.util.spec_from_loader(loader.name, loader)
    module = importlib.util.module_from_spec(specification)
    loader.exec_module(module)
    return module


verify_migration_versions = versions_gate_module()
version_conflicts = verify_migration_versions.version_conflicts


class VersionConflictsTest(unittest.TestCase):
    def test_a_tree_matching_its_base_passes(self):
        names = ["20261028000001_a.sql", "20261029000001_b.sql"]
        self.assertEqual(version_conflicts(names, names), [])

    def test_two_files_with_one_version_here_fail(self):
        here = ["20261029000001_a.sql", "20261029000001_b.sql"]
        self.assertEqual(len(version_conflicts(here, [])), 1)

    def test_a_version_main_gave_another_file_fails(self):
        failures = version_conflicts(["20261029000001_mine.sql"], ["20261029000001_theirs.sql"])
        self.assertEqual(len(failures), 1)

    def test_a_branch_renaming_one_of_a_pair_main_already_repeats_passes(self):
        base = ["20261029000001_first.sql", "20261029000001_second.sql"]
        here = ["20261029000001_first.sql", "20261030000002_second.sql"]
        self.assertEqual(version_conflicts(here, base), [])

    def test_an_unfetched_base_reads_only_this_tree(self):
        self.assertEqual(version_conflicts(["20261029000001_a.sql"], None), [])


if __name__ == "__main__":
    unittest.main()
