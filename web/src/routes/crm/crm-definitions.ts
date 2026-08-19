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
	| { kind: 'stage'; id: string; pipelineID: string }
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
	moveUp: string;
	moveDown: string;
	color: string;
	nameRequired: string;
	readOnly: string;
	saving: string;
	open: string;
	won: string;
	lost: string;
	onHold: string;
};

export const defaultCRMDefinitionsText: CRMDefinitionsText = {
	title: '정의',
	description: 'CRM에서 사용하는 관계처 유형, 진행 유형과 단계, 불발 사유를 관리합니다.',
	organizationTypes: '관계처 유형',
	organizationTypesDescription: '관계처를 분류할 때 사용하는 항목입니다.',
	pipelines: '진행 유형',
	pipelinesDescription: '진행 건의 흐름을 구분하고 각 유형별 단계를 관리합니다.',
	stages: '단계',
	stagesDescription: '이 진행 유형에서 사용하는 단계입니다.',
	lostReasons: '불발 사유',
	lostReasonsDescription: '진행 건이 불발되었을 때 선택하는 사유입니다.',
	add: '추가',
	remove: '삭제',
	moveUp: '위로 이동',
	moveDown: '아래로 이동',
	color: '색상',
	nameRequired: '이름을 입력하세요.',
	readOnly: '관리자만 정의를 수정할 수 있습니다.',
	saving: '저장 중입니다.',
	open: '진행 중',
	won: '성사',
	lost: '불발',
	onHold: '보류'
};

export function cloneCRMVocabulary(vocabulary: CRMVocabulary): CRMVocabulary {
	return {
		organization_types: vocabulary.organization_types.map((definition) => ({ ...definition })),
		pipelines: vocabulary.pipelines.map((pipeline) => ({
			...pipeline,
			stages: pipeline.stages.map((stage) => ({ ...stage }))
		})),
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
	if (collection === 'pipeline') next.pipelines.push({ ...definition, stages: [] });
	if (collection === 'lost_reason') next.lost_reasons.push(definition);
	return next;
}

export function appendCRMStage(
	vocabulary: CRMVocabulary,
	pipelineID: string,
	stage: CRMStageDefinition
): CRMVocabulary {
	const next = cloneCRMVocabulary(vocabulary);
	const pipeline = next.pipelines.find((candidate) => candidate.id === pipelineID);
	if (pipeline) pipeline.stages.push(stage);
	return next;
}

export function updateCRMDefinition(
	vocabulary: CRMVocabulary,
	target: CRMDefinitionTarget,
	patch: Partial<Pick<CRMStageDefinition, 'name' | 'color' | 'outcome'>>
): CRMVocabulary {
	const next = cloneCRMVocabulary(vocabulary);
	if (target.kind === 'stage') {
		const stage = next.pipelines
			.find((pipeline) => pipeline.id === target.pipelineID)
			?.stages.find((candidate) => candidate.id === target.id);
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

export function moveCRMDefinition(
	vocabulary: CRMVocabulary,
	target: CRMDefinitionTarget,
	direction: -1 | 1
): CRMVocabulary {
	const next = cloneCRMVocabulary(vocabulary);
	const definitions = definitionsForTarget(next, target);
	const index = definitions.findIndex((definition) => definition.id === target.id);
	const destination = index + direction;
	if (index < 0 || destination < 0 || destination >= definitions.length) return next;
	const [definition] = definitions.splice(index, 1);
	if (definition) definitions.splice(destination, 0, definition);
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
	return vocabulary.pipelines.find((pipeline) => pipeline.id === target.pipelineID)?.stages ?? [];
}
