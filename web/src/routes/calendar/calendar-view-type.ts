export const ViewType = {
	DAY: 'day',
	WEEK: 'week',
	MONTH: 'month'
} as const;

export type ViewType = (typeof ViewType)[keyof typeof ViewType];
