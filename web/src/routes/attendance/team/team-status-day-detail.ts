import type { TeamStatusPersonDay } from './team-status-table-model';
import type { TeamStatusDayContext } from './team-status-day-context';

export type TeamStatusDayDetail = {
	memberID?: string;
	displayName: string;
	email: string;
	image?: string;
	day: TeamStatusPersonDay;
	context: TeamStatusDayContext;
};
