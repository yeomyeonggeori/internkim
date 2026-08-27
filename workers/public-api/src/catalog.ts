import catalogDocument from '../../../pkg/capabilityprotocol/generated/capability-tools.json';
import type { CallerPermission } from './personal-key';

export type ToolDescriptor = { name: string; sideEffectClass: string };

type ToolCatalog = { protocolVersion: string; tools: ToolDescriptor[] };

const catalog: ToolCatalog = catalogDocument;

const permissionRanks: Record<CallerPermission, number> = { read: 1, write: 2, delete: 3 };

const writingSideEffectClasses = new Set([
	'workspace_write',
	'workspace_calendar',
	'workspace_task',
	'external_write',
	'external_send',
	'external_publish',
	'site_publish',
	'connect',
	'browser',
	'browser_write',
	'handoff',
	'local_file',
	'approval'
]);

export function permissionForTool(descriptor: ToolDescriptor): CallerPermission {
	if (descriptor.sideEffectClass === 'read') return 'read';
	if (writingSideEffectClasses.has(descriptor.sideEffectClass)) return 'write';
	return 'delete';
}

export function toolsReachableBy(permission: CallerPermission): ToolDescriptor[] {
	return catalog.tools.filter(
		(descriptor) => permissionRanks[permissionForTool(descriptor)] <= permissionRanks[permission]
	);
}

export function toolReachableBy(name: string, permission: CallerPermission): ToolDescriptor | undefined {
	const descriptor = catalog.tools.find((candidate) => candidate.name === name);
	if (!descriptor) return undefined;
	if (permissionRanks[permissionForTool(descriptor)] > permissionRanks[permission]) return undefined;
	return descriptor;
}

export const liveParameter = 'live';

export type BaseCatalogAnswer = {
	tools: ToolDescriptor[];
	catalog: {
		source: 'base';
		protocolVersion: string;
		live: string;
		explanation: string;
	};
};

const baseCatalogExplanation =
	'The base catalog is what every company has, answered at the edge from the published protocol. ' +
	'Whether a tool can run right now is known only on your company machine: the companion tools come ' +
	`and go with the companion, and a company may carry tools this list does not. Ask that machine for the ` +
	`set it can run with ?${liveParameter}=true, which costs a round trip.`;

export function baseCatalogAnswer(permission: CallerPermission): BaseCatalogAnswer {
	return {
		tools: toolsReachableBy(permission),
		catalog: {
			source: 'base',
			protocolVersion: catalog.protocolVersion,
			live: `/v1/tools?${liveParameter}=true`,
			explanation: baseCatalogExplanation
		}
	};
}
