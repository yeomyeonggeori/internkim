import importlib.util
import json
import os
import tempfile
import unittest
from unittest import mock


SCRIPT_PATH = os.path.join(os.path.dirname(__file__), 'start-poc.py')
SPECIFICATION = importlib.util.spec_from_file_location('start_poc', SCRIPT_PATH)
START_POC = importlib.util.module_from_spec(SPECIFICATION)
SPECIFICATION.loader.exec_module(START_POC)


class WorkspaceSettingsTest(unittest.TestCase):
    def test_initializes_korean_time_zone_once(self):
        with tempfile.TemporaryDirectory() as workspace:
            START_POC.initialize_workspace_settings(workspace)
            settings_path = os.path.join(workspace, '.admind', 'state', 'workspace-settings.json')
            with open(settings_path) as settings_file:
                self.assertEqual(json.load(settings_file), {'timeZone': 'Asia/Seoul', 'language': 'ko'})

            with open(settings_path, 'w') as settings_file:
                json.dump({'timeZone': 'America/New_York', 'language': 'en'}, settings_file)

            START_POC.initialize_workspace_settings(workspace)
            with open(settings_path) as settings_file:
                self.assertEqual(json.load(settings_file), {'timeZone': 'America/New_York', 'language': 'en'})

    def test_migrates_flow_database_before_recreate(self):
        with tempfile.TemporaryDirectory() as workspace:
            completed_process = mock.Mock(returncode=0)
            with mock.patch.object(START_POC.subprocess, 'run', side_effect=[completed_process, completed_process]) as run:
                START_POC.migrate_flow_database('poc-tenant-01', workspace)

            destination_path = os.path.join(workspace, '.admind', 'flow.sqlite')
            self.assertEqual(run.call_args_list, [
                mock.call([
                    START_POC.CONTAINER,
                    'exec',
                    'poc-tenant-01',
                    'test',
                    '-f',
                    '/root/.internkim/state/flow.sqlite',
                ]),
                mock.call([
                    START_POC.CONTAINER,
                    'cp',
                    'poc-tenant-01:/root/.internkim/state/flow.sqlite',
                    destination_path,
                ], check=True),
            ])

    def test_skips_missing_ephemeral_flow_database(self):
        with tempfile.TemporaryDirectory() as workspace:
            completed_process = mock.Mock(returncode=1)
            with mock.patch.object(START_POC.subprocess, 'run', return_value=completed_process) as run:
                START_POC.migrate_flow_database('poc-tenant-01', workspace)

            run.assert_called_once()
            self.assertFalse(os.path.exists(os.path.join(workspace, '.admind', 'flow.sqlite')))

    def test_preserves_existing_persistent_flow_database(self):
        with tempfile.TemporaryDirectory() as workspace:
            database_path = os.path.join(workspace, '.admind', 'flow.sqlite')
            os.makedirs(os.path.dirname(database_path))
            with open(database_path, 'w') as database_file:
                database_file.write('existing')

            with mock.patch.object(START_POC.subprocess, 'run') as run:
                START_POC.migrate_flow_database('poc-tenant-01', workspace)

            run.assert_not_called()


if __name__ == '__main__':
    unittest.main()
