import catalogDocument from '../../../../../pkg/capabilityprotocol/generated/capability-tools.json';
import descriptorSchema from '../../../../../pkg/capabilityprotocol/generated/json-schema/capability-descriptor.schema.json';
import { z } from 'zod';
import type { PublicAPIPermission } from '$lib/public-api-permission';
import { statesAResultContract } from './catalog/contract';
import { capabilityDescriptorSchema } from './catalog/protocol';

export type ToolDescriptor = z.infer<typeof capabilityDescriptorSchema>;

const toolCatalogSchema = z.strictObject({
	protocolVersion: z.string().trim().min(1),
	tools: z.array(capabilityDescriptorSchema)
});

type ToolCatalog = z.infer<typeof toolCatalogSchema>;

export function parseToolCatalog(document: unknown): ToolCatalog {
	return toolCatalogSchema.parse(document);
}

const catalog = parseToolCatalog(catalogDocument);

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

export function permissionForTool(descriptor: { sideEffectClass: string }): PublicAPIPermission {
	if (readingSideEffectClasses.has(descriptor.sideEffectClass)) return 'read';
	if (writingSideEffectClasses.has(descriptor.sideEffectClass)) return 'write';
	return 'delete';
}

export function toolsReachableBy(permission: PublicAPIPermission): ToolDescriptor[] {
	return reachableTools.filter(
		(descriptor) => permissionRanks[permissionForTool(descriptor)] <= permissionRanks[permission]
	);
}

const toolNamesAModelSees = new Set(
	catalog.tools.filter((tool) => tool.modelVisibility === 'visible').map((tool) => tool.name)
);

export function isSeenByAModel(name: string): boolean {
	return toolNamesAModelSees.has(name);
}

export function toolsAModelReachesWith(permission: PublicAPIPermission): ToolDescriptor[] {
	return toolsReachableBy(permission).filter((descriptor) => isSeenByAModel(descriptor.name));
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
	'Whether a tool can run right now is known only on your company machine, and a company may carry ' +
	`tools this list does not. Ask that machine for the ` +
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
