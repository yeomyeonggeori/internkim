import importlib.util
import json
import os
import stat
import tempfile
import unittest
from pathlib import Path


SCRIPT_PATH = Path(__file__).with_name('refresh_capability_contract.py')
SPECIFICATION = importlib.util.spec_from_file_location('refresh_capability_contract', SCRIPT_PATH)
REFRESH_CAPABILITY_CONTRACT = importlib.util.module_from_spec(SPECIFICATION)
SPECIFICATION.loader.exec_module(REFRESH_CAPABILITY_CONTRACT)


class CapabilityContractRefreshTest(unittest.TestCase):
    def test_refreshes_runtime_and_policy_without_losing_tenant_state(self):
        with tempfile.TemporaryDirectory() as temporary_directory:
            root_path = Path(temporary_directory)
            contract_path = root_path / 'capability-contract.json'
            configuration_root_path = root_path / 'config'
            tenant_path = configuration_root_path / 'tenant_15'
            tenant_path.mkdir(parents=True)
            self.write_json(contract_path, self.contract_document())
            self.write_json(tenant_path / 'runtime.json', self.stale_runtime_document())
            self.write_json(tenant_path / 'policy.json', self.stale_policy_document())
            os.chmod(tenant_path / 'runtime.json', 0o640)

            changed_count = REFRESH_CAPABILITY_CONTRACT.refresh_tenant_configurations(
                contract_path,
                configuration_root_path,
            )

            self.assertEqual(changed_count, 1)
            runtime_document = self.read_json(tenant_path / 'runtime.json')
            policy_document = self.read_json(tenant_path / 'policy.json')
            self.assertEqual(runtime_document['baseURL'], 'http://preserve-tenant-15')
            self.assertEqual(runtime_document['languageModel'], {'model': 'preserve-me'})
            self.assertNotIn('toolNames', runtime_document['capabilities'])
            self.assertEqual(
                runtime_document['capabilities']['toolDescriptors'],
                self.contract_document()['toolDescriptors'],
            )
            self.assertEqual(
                runtime_document['capabilities']['routing']['candidates'],
                self.contract_document()['routingCandidates'],
            )
            self.assertEqual(
                runtime_document['capabilities']['protocolVersion'],
                self.contract_document()['protocolVersion'],
            )
            self.assertEqual(
                runtime_document['capabilities']['aggregateProtocolHash'],
                self.contract_document()['aggregateProtocolHash'],
            )
            self.assertTrue(runtime_document['capabilities']['routing']['localOnly'])
            self.assertEqual(
                stat.S_IMODE(os.stat(tenant_path / 'runtime.json').st_mode),
                0o640,
            )

            resources = {
                entry['resource']: entry
                for entry in policy_document['resourceAccess']
            }
            for resource in self.contract_document()['policyResourceReplacements']:
                self.assertNotIn(resource, resources)
            for resource in [
                'tool:task.update',
                'tool:message.send',
                'tool:channel.update',
                'tool:calendar.list',
                'tool:site.publish',
                'tool:site.repair',
                'custom:tenant-15',
            ]:
                self.assertIn(resource, resources)
            self.assertEqual(resources['custom:tenant-15']['actions'], ['preserve'])
            self.assertEqual(resources['tool:task.list']['circles'], ['custom-circle'])

            first_runtime_document = (tenant_path / 'runtime.json').read_text()
            first_policy_document = (tenant_path / 'policy.json').read_text()
            changed_count = REFRESH_CAPABILITY_CONTRACT.refresh_tenant_configurations(
                contract_path,
                configuration_root_path,
            )
            self.assertEqual(changed_count, 0)
            self.assertEqual((tenant_path / 'runtime.json').read_text(), first_runtime_document)
            self.assertEqual((tenant_path / 'policy.json').read_text(), first_policy_document)

    def test_rejects_duplicate_descriptor_names(self):
        with tempfile.TemporaryDirectory() as temporary_directory:
            contract_path = Path(temporary_directory) / 'capability-contract.json'
            contract_document = self.contract_document()
            contract_document['toolDescriptors'][0]['name'] = contract_document['toolDescriptors'][1]['name']
            self.write_json(contract_path, contract_document)

            with self.assertRaisesRegex(ValueError, 'unique'):
                REFRESH_CAPABILITY_CONTRACT.load_contract(contract_path)

    def test_rejects_missing_or_malformed_protocol_identity(self):
        for field, value in [
            ('protocolVersion', None),
            ('protocolVersion', ' 0.4.0'),
            ('aggregateProtocolHash', 'not-a-hash'),
            ('aggregateProtocolHash', 'A' * 64),
        ]:
            with self.subTest(field=field):
                with tempfile.TemporaryDirectory() as temporary_directory:
                    contract_path = Path(temporary_directory) / 'capability-contract.json'
                    contract_document = self.contract_document()
                    contract_document[field] = value
                    self.write_json(contract_path, contract_document)

                    with self.assertRaisesRegex(ValueError, field):
                        REFRESH_CAPABILITY_CONTRACT.load_contract(contract_path)

    def test_rejects_legacy_contract_version(self):
        with tempfile.TemporaryDirectory() as temporary_directory:
            contract_path = Path(temporary_directory) / 'capability-contract.json'
            contract_document = self.contract_document()
            contract_document['version'] = 2
            self.write_json(contract_path, contract_document)

            with self.assertRaisesRegex(ValueError, 'version must be 3'):
                REFRESH_CAPABILITY_CONTRACT.load_contract(contract_path)

    def contract_document(self):
        tool_names = [
            'task.update',
            'message.send',
            'channel.update',
            'calendar.list',
            'site.publish',
        ]
        return {
            'version': 3,
            'protocolVersion': '0.4.0',
            'aggregateProtocolHash': 'a' * 64,
            'toolDescriptors': [
                {'name': tool_name, 'version': '1'}
                for tool_name in tool_names
            ],
            'routingCandidates': ['companion', 'device'],
            'policyResourceReplacements': {
                'tool:flow.task.update': 'tool:task.update',
                'tool:platform.message.send': 'tool:message.send',
                'tool:mattermost.channel.update': 'tool:channel.update',
                'tool:calendar.event.list': 'tool:calendar.list',
                'tool:site.app.publish': 'tool:site.publish',
            },
            'policyResourceDefaults': [
                {
                    'resource': 'tool:task.list',
                    'actions': ['execute'],
                    'circles': ['staff'],
                },
                {
                    'resource': 'tool:site.repair',
                    'actions': ['execute'],
                    'circles': ['staff'],
                },
            ],
        }

    def stale_runtime_document(self):
        return {
            'baseURL': 'http://preserve-tenant-15',
            'capabilities': {
                'toolNames': ['flow.task.update'],
                'toolDescriptors': [{'name': 'flow.task.update', 'version': '1'}],
                'routing': {
                    'candidates': ['legacy'],
                    'localOnly': True,
                },
            },
            'languageModel': {'model': 'preserve-me'},
        }

    def stale_policy_document(self):
        return {
            'people': [{'personID': 'preserve-person'}],
            'resourceAccess': [
                {'resource': 'tool:flow.task.update', 'actions': ['execute'], 'circles': ['staff']},
                {'resource': 'tool:platform.message.send', 'actions': ['execute'], 'circles': ['staff']},
                {'resource': 'tool:mattermost.channel.update', 'actions': ['execute'], 'circles': ['admin']},
                {'resource': 'tool:calendar.event.list', 'actions': ['execute'], 'circles': ['staff']},
                {'resource': 'tool:site.app.publish', 'actions': ['execute'], 'circles': ['staff']},
                {'resource': 'tool:task.list', 'actions': ['execute'], 'circles': ['custom-circle']},
                {'resource': 'custom:tenant-15', 'actions': ['preserve'], 'circles': ['custom-circle']},
            ],
        }

    def write_json(self, path, document):
        path.write_text(json.dumps(document, ensure_ascii=False, indent=2) + '\n')

    def read_json(self, path):
        return json.loads(path.read_text())


if __name__ == '__main__':
    unittest.main()
