export type TeamCardStatus = 'working' | 'done' | 'away' | 'notStarted';

export function memberStatusOf(status: TeamCardStatus): 'working' | 'done' | 'away' | 'not_started' {
	return status === 'notStarted' ? 'not_started' : status;
}
