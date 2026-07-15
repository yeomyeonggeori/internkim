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
    def test_tenant_entrypoint_routes_mattermost_actions_through_public_url(self):
        entrypoint_path = os.path.join(os.path.dirname(__file__), 'tenant', 'entrypoint.sh')
        with open(entrypoint_path) as entrypoint_file:
            entrypoint = entrypoint_file.read()

        self.assertIn('flowPublicURL="$(cat /root/.internkim/env/flow-public-url', entrypoint)
        self.assertEqual(entrypoint.count('--mattermost-interactive-base-url "${flowPublicURL:-}"'), 2)
        self.assertEqual(entrypoint.count('--mattermost-interactive-token "${mattermostInteractiveTokenPath}"'), 2)

    def test_refreshes_capability_contract_before_resolving_container_ips(self):
        with mock.patch.object(START_POC, 'refresh_capability_contract') as refresh_contract:
            def inspect_container(name):
                self.assertTrue(refresh_contract.called)
                return {'poc-postgres': 'postgres-ip', 'poc-mattermost': 'mattermost-ip'}[name]

            with mock.patch.object(START_POC, 'container_ip', side_effect=inspect_container):
                with mock.patch.object(START_POC, 'patch_ips_in_configs'):
                    with mock.patch.object(START_POC, 'start_tenant'):
                        with mock.patch('builtins.print'):
                            START_POC.start_all_tenants(1)

    def test_runs_capability_contract_refresh_script_when_contract_exists(self):
        with tempfile.TemporaryDirectory() as temporary_directory:
            contract_path = os.path.join(temporary_directory, START_POC.CAPABILITY_CONTRACT_FILENAME)
            script_path = os.path.join(temporary_directory, START_POC.CAPABILITY_CONTRACT_REFRESH_SCRIPT)
            with open(contract_path, 'w'):
                pass
            with open(script_path, 'w'):
                pass
            with mock.patch.object(START_POC, 'BASE', temporary_directory):
                with mock.patch.object(START_POC.subprocess, 'run') as run:
                    START_POC.refresh_capability_contract()

            run.assert_called_once_with([
                START_POC.sys.executable,
                script_path,
                '--contract',
                contract_path,
                '--config-root',
                os.path.join(temporary_directory, 'config'),
            ], check=True)

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

    def test_converts_only_tenant_internkim_account_to_bot(self):
        completed_process = mock.Mock(returncode=0)
        with mock.patch.object(START_POC.subprocess, 'run', return_value=completed_process) as run:
            START_POC.ensure_mattermost_bot(15)

        run.assert_called_once_with([
            START_POC.CONTAINER,
            'exec',
            'poc-mattermost',
            'mmctl',
            '--local',
            'user',
            'convert',
            'internkim15',
            '--bot',
        ], capture_output=True, text=True)

    def test_verifies_tenant_token_belongs_to_expected_bot(self):
        completed_process = mock.Mock(
            returncode=0,
            stdout=json.dumps({'username': 'internkim15', 'is_bot': True}),
            stderr='',
        )
        with mock.patch.object(START_POC.subprocess, 'run', return_value=completed_process) as run:
            START_POC.verify_mattermost_bot('poc-tenant-15', 'internkim15')

        arguments = run.call_args.args[0]
        self.assertEqual(arguments[:4], [START_POC.CONTAINER, 'exec', 'poc-tenant-15', 'sh'])
        self.assertIn('/secrets/mattermost-bot-token', arguments[-1])
        self.assertNotIn('admin15', ' '.join(arguments))

    def test_rejects_human_tenant_token_user(self):
        completed_process = mock.Mock(
            returncode=0,
            stdout=json.dumps({'username': 'internkim15', 'is_bot': False}),
            stderr='',
        )
        with mock.patch.object(START_POC.subprocess, 'run', return_value=completed_process):
            with self.assertRaisesRegex(RuntimeError, 'Mattermost account is not a bot: internkim15'):
                START_POC.verify_mattermost_bot('poc-tenant-15', 'internkim15')


if __name__ == '__main__':
    unittest.main()
