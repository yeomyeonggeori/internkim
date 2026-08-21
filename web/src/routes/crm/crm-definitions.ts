import type {
	CRMDefinition,
	CRMPipelineDefinition,
	CRMVocabulary
} from './crm-api-types';

export type { CRMDefinition, CRMPipelineDefinition, CRMVocabulary };

export type CRMDefinitionTarget =
	| { kind: 'organization_type'; id: string }
	| { kind: 'pipeline'; id: string };

export type CRMDefinitionDeleteRequest = CRMDefinitionTarget;

export type CRMDefinitionCollection = 'organization_type' | 'pipeline';

export type CRMDefinitionsText = {
	title: string;
	description: string;
	organizationTypes: string;
	organizationTypesDescription: string;
	pipelines: string;
	pipelinesDescription: string;
	add: string;
	remove: string;
	color: string;
	nameRequired: string;
	readOnly: string;
	saving: string;
};

export function cloneCRMVocabulary(vocabulary: CRMVocabulary): CRMVocabulary {
	return {
		organization_types: vocabulary.organization_types.map((definition) => ({ ...definition })),
		pipelines: vocabulary.pipelines.map((pipeline) => ({ ...pipeline }))
	};
}

export function appendCRMDefinition(
	vocabulary: CRMVocabulary,
	collection: CRMDefinitionCollection,
	definition: CRMDefinition
): CRMVocabulary {
	const next = cloneCRMVocabulary(vocabulary);
	if (collection === 'organization_type') next.organization_types.push(definition);
	if (collection === 'pipeline') next.pipelines.push(definition);
	return next;
}

export function updateCRMDefinition(
	vocabulary: CRMVocabulary,
	target: CRMDefinitionTarget,
	patch: Partial<Pick<CRMDefinition, 'name' | 'color'>>
): CRMVocabulary {
	const next = cloneCRMVocabulary(vocabulary);
	const definition = definitionAt(next, target);
	if (!definition) return next;
	if (patch.name !== undefined) definition.name = patch.name;
	if (patch.color !== undefined) definition.color = patch.color;
	return next;
}

export function removeCRMDefinition(
	vocabulary: CRMVocabulary,
	target: CRMDefinitionTarget
): CRMVocabulary {
	const next = cloneCRMVocabulary(vocabulary);
	const definitions = definitionsForTarget(next, target);
	const index = definitions.findIndex((definition) => definition.id === target.id);
	if (index >= 0) definitions.splice(index, 1);
	return next;
}

function definitionAt(
	vocabulary: CRMVocabulary,
	target: CRMDefinitionTarget
): CRMDefinition | CRMPipelineDefinition | undefined {
	return definitionsForTarget(vocabulary, target).find((definition) => definition.id === target.id);
}

function definitionsForTarget(
	vocabulary: CRMVocabulary,
	target: CRMDefinitionTarget
): Array<CRMDefinition | CRMPipelineDefinition> {
	return target.kind === 'organization_type' ? vocabulary.organization_types : vocabulary.pipelines;
}
