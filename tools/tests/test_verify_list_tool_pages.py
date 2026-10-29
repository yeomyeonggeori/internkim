import importlib.machinery
import importlib.util
import json
import pathlib
import sys
import unittest


REPOSITORY_ROOT = pathlib.Path(__file__).resolve().parents[2]
sys.path.insert(0, str(REPOSITORY_ROOT / "tools"))


def pages_gate_module():
    script = REPOSITORY_ROOT / "tools" / "verify-list-tool-pages"
    loader = importlib.machinery.SourceFileLoader("verify_list_tool_pages", str(script))
    specification = importlib.util.spec_from_loader(loader.name, loader)
    module = importlib.util.module_from_spec(specification)
    loader.exec_module(module)
    return module


verify_list_tool_pages = pages_gate_module()


def tool(name, input_properties, output_properties):
    return {
        "name": name,
        "inputSchema": json.dumps({"properties": input_properties}),
        "outputSchema": {"properties": output_properties},
    }


def fault_kinds(descriptor):
    return {kind for (kind, _), _ in verify_list_tool_pages.observed_faults(descriptor)}


class ObservedFaultsTest(unittest.TestCase):
    def test_a_tool_paged_the_agreed_way_has_no_fault(self):
        descriptor = tool(
            "thing_list",
            {"limit": {"type": "integer", "maximum": 100}, "cursor": {"type": "string"}},
            {"things": {}, "totalCount": {}, "nextCursor": {}},
        )

        self.assertEqual(fault_kinds(descriptor), set())

    def test_a_cap_without_a_cursor_is_a_cut_answer(self):
        descriptor = tool("thing_list", {"limit": {"type": "integer", "maximum": 48}}, {"things": {}})

        self.assertEqual(fault_kinds(descriptor), {"cut"})

    def test_a_cursor_needs_a_bounded_page_a_next_cursor_and_a_total(self):
        descriptor = tool("thing_list", {"limit": {"type": "integer"}, "cursor": {"type": "string"}}, {"things": {}})

        self.assertEqual(fault_kinds(descriptor), {"unbounded", "next", "total"})

    def test_a_tool_with_neither_limit_nor_cursor_answers_everything_and_is_not_checked(self):
        descriptor = tool("team_list", {}, {"teams": {}})

        self.assertEqual(fault_kinds(descriptor), set())


class StaleEntryFaultsTest(unittest.TestCase):
    def test_an_entry_for_a_fault_that_was_fixed_must_be_removed(self):
        faults = list(verify_list_tool_pages.stale_entry_faults({("cut", "task_list")}, set()))

        self.assertEqual(len(faults), 1)
        self.assertIn("task_list", faults[0])


if __name__ == "__main__":
    unittest.main()
