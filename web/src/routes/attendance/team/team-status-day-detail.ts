import type { TeamStatusPersonDay } from './team-status-table-model';
import type { TeamStatusDayContext } from './team-status-day-context';

export type TeamStatusDayDetail = {
	displayName: string;
	email: string;
	mattermostUsername?: string;
	day: TeamStatusPersonDay;
	context: TeamStatusDayContext;
};
