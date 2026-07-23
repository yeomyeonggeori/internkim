#!/usr/bin/env python3
import argparse
import copy
import json
import os
import re
import stat
import tempfile
from pathlib import Path


def load_json(path):
    with Path(path).open() as document_file:
        return json.load(document_file)


def require_string_list(document, key):
    values = document.get(key)
    if not isinstance(values, list) or not values:
        raise ValueError(f'{key} must be a non-empty list')
    if not all(isinstance(value, str) and value for value in values):
        raise ValueError(f'{key} must contain non-empty strings')
    return values


def validate_tool_descriptors(contract):
    descriptors = contract.get('toolDescriptors')
    if not isinstance(descriptors, list) or not descriptors:
        raise ValueError('toolDescriptors must be a non-empty list')
    descriptor_names = [
        descriptor.get('name') if isinstance(descriptor, dict) else None
        for descriptor in descriptors
    ]
    if not all(isinstance(name, str) and name for name in descriptor_names):
        raise ValueError('tool descriptors must have names')
    if len(descriptor_names) != len(set(descriptor_names)):
        raise ValueError('tool descriptor names must be unique')


def validate_policy_contract(contract):
    replacements = contract.get('policyResourceReplacements')
    if not isinstance(replacements, dict):
        raise ValueError('policyResourceReplacements must be an object')
    if not all(isinstance(source, str) and isinstance(target, str) for source, target in replacements.items()):
        raise ValueError('policy resource replacements must map strings to strings')
    defaults = contract.get('policyResourceDefaults')
    if not isinstance(defaults, list):
        raise ValueError('policyResourceDefaults must be a list')
    resources = [entry.get('resource') if isinstance(entry, dict) else None for entry in defaults]
    if not all(isinstance(resource, str) and resource for resource in resources):
        raise ValueError('policy resource defaults must have resources')
    if len(resources) != len(set(resources)):
        raise ValueError('policy resource defaults must be unique')


def validate_protocol_identity(contract):
    protocol_version = contract.get('protocolVersion')
    aggregate_protocol_hash = contract.get('aggregateProtocolHash')
    if not isinstance(protocol_version, str) or not protocol_version or protocol_version != protocol_version.strip():
        raise ValueError('protocolVersion must be a non-empty trimmed string')
    if not isinstance(aggregate_protocol_hash, str) or not re.fullmatch(r'[0-9a-f]{64}', aggregate_protocol_hash):
        raise ValueError('aggregateProtocolHash must be a 64-character lowercase hexadecimal hash')


def load_contract(path):
    contract = load_json(path)
    if not isinstance(contract, dict) or contract.get('version') != 3:
        raise ValueError('capability contract version must be 3')
    validate_protocol_identity(contract)
    validate_tool_descriptors(contract)
    require_string_list(contract, 'routingCandidates')
    validate_policy_contract(contract)
    return contract


def require_object(document, key, document_name):
    value = document.get(key)
    if not isinstance(value, dict):
        raise ValueError(f'{document_name}.{key} must be an object')
    return value


DIRECT_LLMD_ENDPOINT = 'http://internkim/_internkim/llmd'
CAPABILITY_SOCKET_PATH = '/run/internkim/capability.sock'


def refreshed_runtime_document(runtime_document, contract):
    if not isinstance(runtime_document, dict):
        raise ValueError('runtime document must be an object')
    refreshed_document = copy.deepcopy(runtime_document)
    capability_configuration = require_object(refreshed_document, 'capabilities', 'runtime')
    routing_configuration = require_object(capability_configuration, 'routing', 'runtime.capabilities')
    capability_configuration.pop('toolNames', None)
    capability_configuration['protocolVersion'] = contract['protocolVersion']
    capability_configuration['aggregateProtocolHash'] = contract['aggregateProtocolHash']
    capability_configuration['toolDescriptors'] = copy.deepcopy(contract['toolDescriptors'])
    routing_configuration['candidates'] = copy.deepcopy(contract['routingCandidates'])
    heal_language_model_llmd(refreshed_document)
    return refreshed_document


def heal_language_model_llmd(runtime_document):
    language_model = runtime_document.get('languageModel')
    if not isinstance(language_model, dict):
        return
    llmd_section = language_model.get('llmd')
    if not isinstance(llmd_section, dict):
        llmd_section = {
            'endpoint': DIRECT_LLMD_ENDPOINT,
            'unixSocketPath': CAPABILITY_SOCKET_PATH,
            'authKeyPath': '',
            'executionMode': 'auto',
            'localOnly': False,
        }
        language_model['llmd'] = llmd_section
    if not str(llmd_section.get('endpoint') or '').strip():
        llmd_section['endpoint'] = DIRECT_LLMD_ENDPOINT
        if not str(llmd_section.get('unixSocketPath') or '').strip():
            llmd_section['unixSocketPath'] = CAPABILITY_SOCKET_PATH
    if str(language_model.get('defaultProvider') or '').strip() in ('', 'capabilityLLM'):
        language_model['defaultProvider'] = 'llmd'


def validated_resource_access(policy_document):
    resource_access = policy_document.get('resourceAccess')
    if not isinstance(resource_access, list):
        raise ValueError('policy.resourceAccess must be a list')
    for entry in resource_access:
        if not isinstance(entry, dict) or not isinstance(entry.get('resource'), str):
            raise ValueError('policy resource access entries must have resources')
    return resource_access


def migrate_policy_resources(resource_access, replacements):
    current_resources = {
        entry['resource']
        for entry in resource_access
        if entry['resource'] not in replacements
    }
    migrated_entries = []
    migrated_resources = set()
    for entry in resource_access:
        source_resource = entry['resource']
        target_resource = replacements.get(source_resource, source_resource)
        if source_resource != target_resource and target_resource in current_resources:
            continue
        if target_resource in migrated_resources:
            continue
        migrated_entry = copy.deepcopy(entry)
        migrated_entry['resource'] = target_resource
        migrated_entries.append(migrated_entry)
        migrated_resources.add(target_resource)
    return migrated_entries, migrated_resources


def refreshed_policy_document(policy_document, contract):
    if not isinstance(policy_document, dict):
        raise ValueError('policy document must be an object')
    refreshed_document = copy.deepcopy(policy_document)
    resource_access = validated_resource_access(refreshed_document)
    migrated_entries, migrated_resources = migrate_policy_resources(
        resource_access,
        contract['policyResourceReplacements'],
    )
    for default_entry in contract['policyResourceDefaults']:
        if default_entry['resource'] not in migrated_resources:
            migrated_entries.append(copy.deepcopy(default_entry))
            migrated_resources.add(default_entry['resource'])
    refreshed_document['resourceAccess'] = migrated_entries
    return refreshed_document


def serialized_json(document):
    return json.dumps(document, ensure_ascii=False, indent=2) + '\n'


def write_json_if_changed(path, document):
    path = Path(path)
    updated_document = serialized_json(document)
    if path.read_text() == updated_document:
        return False
    file_mode = stat.S_IMODE(path.stat().st_mode)
    descriptor, temporary_path = tempfile.mkstemp(prefix=path.name + '.', dir=path.parent)
    try:
        os.fchmod(descriptor, file_mode)
        with os.fdopen(descriptor, 'w') as temporary_file:
            temporary_file.write(updated_document)
            temporary_file.flush()
            os.fsync(temporary_file.fileno())
        os.replace(temporary_path, path)
    except Exception:
        if os.path.exists(temporary_path):
            os.unlink(temporary_path)
        raise
    return True


def refresh_tenant_configuration(tenant_path, contract):
    runtime_path = tenant_path / 'runtime.json'
    policy_path = tenant_path / 'policy.json'
    runtime_document = refreshed_runtime_document(load_json(runtime_path), contract)
    policy_document = refreshed_policy_document(load_json(policy_path), contract)
    runtime_changed = write_json_if_changed(runtime_path, runtime_document)
    policy_changed = write_json_if_changed(policy_path, policy_document)
    return runtime_changed or policy_changed


def refresh_tenant_configurations(contract_path, configuration_root_path):
    contract = load_contract(contract_path)
    tenant_paths = sorted(
        path
        for path in Path(configuration_root_path).glob('tenant_[0-9]*')
        if path.is_dir() and re.fullmatch(r'tenant_[0-9]+', path.name)
    )
    return sum(refresh_tenant_configuration(path, contract) for path in tenant_paths)


def parse_arguments():
    parser = argparse.ArgumentParser()
    parser.add_argument('--contract', default='capability-contract.json')
    parser.add_argument('--config-root', default='config')
    return parser.parse_args()


def main():
    arguments = parse_arguments()
    changed_count = refresh_tenant_configurations(arguments.contract, arguments.config_root)
    print(f'Refreshed capability contracts for {changed_count} tenant configurations')


if __name__ == '__main__':
    main()
