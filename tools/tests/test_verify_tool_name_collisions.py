import importlib.machinery
import importlib.util
import pathlib
import sys
import unittest
import unittest.mock


REPOSITORY_ROOT = pathlib.Path(__file__).resolve().parents[2]
sys.path.insert(0, str(REPOSITORY_ROOT / "tools"))

import tool_name_sources  # noqa: E402


def collision_gate_module():
    script = REPOSITORY_ROOT / "tools" / "verify-tool-name-collisions"
    loader = importlib.machinery.SourceFileLoader("verify_tool_name_collisions", str(script))
    specification = importlib.util.spec_from_loader(loader.name, loader)
    module = importlib.util.module_from_spec(specification)
    loader.exec_module(module)
    return module


verify_tool_name_collisions = collision_gate_module()


class CollisionsBetweenTest(unittest.TestCase):
    def test_a_catalog_tool_a_local_tool_also_serves_is_a_collision(self):
        collisions = verify_tool_name_collisions.collisions_between(
            {"task_add", "event_add"}, {"task_add"}, set()
        )

        self.assertEqual(len(collisions), 1)
        self.assertIn("task_add", collisions[0])
        self.assertIn("a blueclaw local tool", collisions[0])

    def test_a_catalog_tool_a_kernel_tool_also_serves_is_a_collision(self):
        collisions = verify_tool_name_collisions.collisions_between(
            {"file_read"}, set(), {"file_read"}
        )

        self.assertEqual(len(collisions), 1)
        self.assertIn("a bluecollar kernel tool", collisions[0])

    def test_a_collision_says_how_a_deliberate_migration_step_declares_itself(self):
        collisions = verify_tool_name_collisions.collisions_between(
            {"task_add"}, {"task_add"}, set()
        )

        self.assertIn("TOOLS_BEING_MOVED", collisions[0])

    def test_a_tool_being_moved_is_allowed_to_exist_on_both_sides(self):
        with unittest.mock.patch.dict(
            verify_tool_name_collisions.TOOLS_BEING_MOVED, {"task_add": "moving for this test"}
        ):
            collisions = verify_tool_name_collisions.collisions_between(
                {"task_add"}, {"task_add"}, {"task_add"}
            )

        self.assertEqual(collisions, [])

    def test_nothing_is_being_moved(self):
        self.assertEqual(verify_tool_name_collisions.TOOLS_BEING_MOVED, {})

    def test_a_name_neither_native_side_serves_is_not_a_collision(self):
        collisions = verify_tool_name_collisions.collisions_between(
            {"task_add"}, {"memory_search"}, {"bash"}
        )

        self.assertEqual(collisions, [])


class ModelVisibleCatalogNamesTest(unittest.TestCase):
    def test_a_hidden_catalog_tool_is_not_a_name_the_model_competes_for(self):
        hidden = tool_name_sources.model_hidden_tool_names()

        self.assertTrue(hidden, "the catalog declares no hidden tool, so this test proves nothing")
        self.assertEqual(hidden & tool_name_sources.model_visible_catalog_names(), set())

    def test_every_tool_being_moved_still_has_the_native_twin_it_names(self):
        native = tool_name_sources.blueclaw_local_tool_names() | tool_name_sources.kernel_tool_names()

        for tool_name in verify_tool_name_collisions.TOOLS_BEING_MOVED:
            self.assertIn(
                tool_name,
                native,
                f"{tool_name} has no native twin left, so it should leave TOOLS_BEING_MOVED",
            )


if __name__ == "__main__":
    unittest.main()
