export type TaskBoardParticipantScope = 'self' | 'other' | 'everyone';

export function taskBoardParticipantScope(
	participantFilterIDs: string[],
	viewerMemberID: string | undefined
): TaskBoardParticipantScope {
	if (participantFilterIDs.length === 0) return 'everyone';
	if (!viewerMemberID) return 'other';
	return participantFilterIDs.includes(viewerMemberID) ? 'self' : 'other';
}

export function canCreateTaskInColumn(status: string, scope: TaskBoardParticipantScope): boolean {
	return scope !== 'other' || status === 'requested';
}

export function shouldHideEmptyRequestColumn(scope: TaskBoardParticipantScope): boolean {
	return scope === 'self';
}
