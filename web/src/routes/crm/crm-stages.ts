import type { CRMPipelineStage } from './crm-types';

export type CRMStageCatalogEntry = CRMPipelineStage & { color: string };

export const crmStages: CRMStageCatalogEntry[] = [
	{ stage: 'waiting', label: 'waiting', position: 1, outcome: 'open', color: '#64748b' },
	{ stage: 'in_progress', label: 'in_progress', position: 2, outcome: 'open', color: '#2563eb' },
	{ stage: 'review', label: 'review', position: 3, outcome: 'open', color: '#7c3aed' },
	{ stage: 'done', label: 'done', position: 4, outcome: 'won', color: '#16a34a' },
	{ stage: 'on_hold', label: 'on_hold', position: 5, outcome: 'on_hold', color: '#f59e0b' },
	{ stage: 'lost', label: 'lost', position: 6, outcome: 'lost', color: '#dc2626' }
];
