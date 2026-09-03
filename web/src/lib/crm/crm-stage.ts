export const crmStageKeys = ['waiting', 'in_progress', 'review', 'done', 'on_hold', 'lost'] as const;

export type CRMStage = (typeof crmStageKeys)[number];

export type CRMStageOutcome = 'open' | 'won' | 'lost' | 'on_hold';

export const crmStageOutcomes: Record<CRMStage, CRMStageOutcome> = {
	waiting: 'open',
	in_progress: 'open',
	review: 'open',
	done: 'won',
	on_hold: 'on_hold',
	lost: 'lost'
};

export function isCRMStage(value: string): value is CRMStage {
	return (crmStageKeys as readonly string[]).includes(value);
}

export function crmStageSettles(stage: CRMStage): boolean {
	const outcome = crmStageOutcomes[stage];
	return outcome === 'won' || outcome === 'lost';
}
