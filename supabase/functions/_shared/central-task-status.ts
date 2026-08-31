export const centralTaskStatuses = [
	'requested',
	'planned',
	'in_progress',
	'completed',
	'paused',
	'rejected',
	'stopped'
] as const;

export type CentralTaskStatus = (typeof centralTaskStatuses)[number];
