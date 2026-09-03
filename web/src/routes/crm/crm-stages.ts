import { crmStageKeys, crmStageOutcomes, type CRMStage } from '$lib/crm/crm-stage';
import type { CRMPipelineStage } from './crm-types';

export type CRMStageCatalogEntry = CRMPipelineStage & { color: string };

const stageColors: Record<CRMStage, string> = {
	waiting: '#64748b',
	in_progress: '#2563eb',
	review: '#7c3aed',
	done: '#16a34a',
	on_hold: '#f59e0b',
	lost: '#dc2626'
};

export const crmStages: CRMStageCatalogEntry[] = crmStageKeys.map((stage, position) => ({
	stage,
	label: stage,
	position: position + 1,
	outcome: crmStageOutcomes[stage],
	color: stageColors[stage]
}));
