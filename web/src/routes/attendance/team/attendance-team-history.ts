import { pushState, replaceState } from '$app/navigation';
import { page } from '$app/state';

export function teamKeyInHistory(): string | undefined {
	return page.state.openAttendanceTeamKey;
}

export function rememberOpenedTeam(teamKey: string): void {
	const state = { ...page.state, openAttendanceTeamKey: teamKey };
	if (teamKeyInHistory() === undefined) pushState('', state);
	else replaceState('', state);
}

export function leaveOpenedTeamThroughHistory(): boolean {
	if (teamKeyInHistory() === undefined) return false;
	history.back();
	return true;
}
