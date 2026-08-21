import type {
	CRMDefinition,
	CRMPipelineDefinition,
	CRMStageDefinition,
	CRMVocabulary
} from './crm-api-types';

export type { CRMDefinition, CRMPipelineDefinition, CRMStageDefinition, CRMVocabulary };
export type CRMStageOutcome = CRMStageDefinition['outcome'];

export type CRMDefinitionTarget =
	| { kind: 'organization_type'; id: string }
	| { kind: 'pipeline'; id: string }
	| { kind: 'stage'; id: string }
	| { kind: 'lost_reason'; id: string };

export type CRMDefinitionDeleteRequest = CRMDefinitionTarget;

export type CRMDefinitionCollection = 'organization_type' | 'pipeline' | 'lost_reason';

export type CRMDefinitionsText = {
	title: string;
	description: string;
	organizationTypes: string;
	organizationTypesDescription: string;
	pipelines: string;
	pipelinesDescription: string;
	stages: string;
	stagesDescription: string;
	lostReasons: string;
	lostReasonsDescription: string;
	add: string;
	remove: string;
	color: string;
	nameRequired: string;
	readOnly: string;
	saving: string;
	open: string;
	won: string;
	lost: string;
	onHold: string;
};

export function cloneCRMVocabulary(vocabulary: CRMVocabulary): CRMVocabulary {
	return {
		organization_types: vocabulary.organization_types.map((definition) => ({ ...definition })),
		pipelines: vocabulary.pipelines.map((pipeline) => ({ ...pipeline })),
		stages: vocabulary.stages.map((stage) => ({ ...stage })),
		lost_reasons: vocabulary.lost_reasons.map((definition) => ({ ...definition }))
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
	if (collection === 'lost_reason') next.lost_reasons.push(definition);
	return next;
}

export function appendCRMStage(vocabulary: CRMVocabulary, stage: CRMStageDefinition): CRMVocabulary {
	const next = cloneCRMVocabulary(vocabulary);
	next.stages.push(stage);
	return next;
}

export function updateCRMDefinition(
	vocabulary: CRMVocabulary,
	target: CRMDefinitionTarget,
	patch: Partial<Pick<CRMStageDefinition, 'name' | 'color' | 'outcome'>>
): CRMVocabulary {
	const next = cloneCRMVocabulary(vocabulary);
	if (target.kind === 'stage') {
		const stage = next.stages.find((candidate) => candidate.id === target.id);
		if (!stage) return next;
		if (patch.name !== undefined) stage.name = patch.name;
		if (patch.color !== undefined) stage.color = patch.color;
		if (patch.outcome !== undefined) stage.outcome = patch.outcome;
		return next;
	}
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
): CRMDefinition | CRMPipelineDefinition | CRMStageDefinition | undefined {
	return definitionsForTarget(vocabulary, target).find((definition) => definition.id === target.id);
}

function definitionsForTarget(
	vocabulary: CRMVocabulary,
	target: CRMDefinitionTarget
): Array<CRMDefinition | CRMPipelineDefinition | CRMStageDefinition> {
	if (target.kind === 'organization_type') return vocabulary.organization_types;
	if (target.kind === 'pipeline') return vocabulary.pipelines;
	if (target.kind === 'lost_reason') return vocabulary.lost_reasons;
	return vocabulary.stages;
}
