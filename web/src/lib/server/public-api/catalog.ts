import catalogDocument from '../../../../../pkg/capabilityprotocol/generated/capability-tools.json';
import descriptorSchema from '../../../../../pkg/capabilityprotocol/generated/json-schema/capability-descriptor.schema.json';
import type { PublicAPIPermission } from '$lib/public-api-permission';
import { statesAResultContract } from './catalog/contract';

export type ToolDescriptor = {
	name: string;
	sideEffectClass: string;
	description: string;
	inputSchema: Record<string, unknown>;
	outputSchema: Record<string, unknown>;
};

type CatalogEntry = ToolDescriptor & { answeredBy: string; resultContract?: unknown };

type ToolCatalog = { protocolVersion: string; tools: CatalogEntry[] };

const catalog: ToolCatalog = catalogDocument;

const sideEffectClassVocabulary: string[] = descriptorSchema.properties.sideEffectClass.enum;

const reachableTools: ToolDescriptor[] = catalog.tools.filter(
	(tool) => statesAResultContract(tool) && tool.answeredBy !== 'local'
);

const permissionRanks: Record<PublicAPIPermission, number> = { read: 1, write: 2, delete: 3 };

const readingSideEffectClasses = new Set(['read', 'computation']);
const deletingSideEffectClasses = new Set(['destructive']);

const writingSideEffectClasses = new Set(
	sideEffectClassVocabulary.filter(
		(sideEffectClass) =>
			!readingSideEffectClasses.has(sideEffectClass) &&
			!deletingSideEffectClasses.has(sideEffectClass)
	)
);

export type Answerer = 'record' | 'company' | 'local';

export function answererOfTool(name: string): Answerer | undefined {
	const descriptor = catalog.tools.find((candidate) => candidate.name === name);
	if (!descriptor) return undefined;
	return descriptor.answeredBy as Answerer;
}

export function toolNamesAnsweredBy(answerer: Answerer): string[] {
	return catalog.tools.filter((tool) => tool.answeredBy === answerer).map((tool) => tool.name);
}

export function destroysSomething(descriptor: Pick<ToolDescriptor, 'sideEffectClass'>): boolean {
	return deletingSideEffectClasses.has(descriptor.sideEffectClass);
}

export function permissionForTool(descriptor: Pick<ToolDescriptor, 'sideEffectClass'>): PublicAPIPermission {
	if (readingSideEffectClasses.has(descriptor.sideEffectClass)) return 'read';
	if (writingSideEffectClasses.has(descriptor.sideEffectClass)) return 'write';
	return 'delete';
}

export function toolsReachableBy(permission: PublicAPIPermission): ToolDescriptor[] {
	return reachableTools.filter(
		(descriptor) => permissionRanks[permissionForTool(descriptor)] <= permissionRanks[permission]
	);
}

export function toolReachableBy(name: string, permission: PublicAPIPermission): ToolDescriptor | undefined {
	const descriptor = reachableTools.find((candidate) => candidate.name === name);
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
	'The base catalog is what every company has, answered from the published protocol. ' +
	'Whether a tool can run right now is known only on your company machine: the companion tools come ' +
	`and go with the companion, and a company may carry tools this list does not. Ask that machine for the ` +
	`set it can run with ?${liveParameter}=true, which costs a round trip.`;

export function baseCatalogAnswer(permission: PublicAPIPermission): BaseCatalogAnswer {
	return {
		tools: toolsReachableBy(permission),
		catalog: {
			source: 'base',
			protocolVersion: catalog.protocolVersion,
			live: `/api/v1/tools?${liveParameter}=true`,
			explanation: baseCatalogExplanation
		}
	};
}
