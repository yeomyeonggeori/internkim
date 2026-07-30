export type FlowBoardParticipantScope = 'self' | 'other' | 'everyone';

export function flowBoardParticipantScope(
	participantFilterIDs: string[],
	viewerMemberID: string | undefined
): FlowBoardParticipantScope {
	if (participantFilterIDs.length === 0) return 'everyone';
	if (!viewerMemberID) return 'other';
	return participantFilterIDs.includes(viewerMemberID) ? 'self' : 'other';
}

export function canCreateFlowTaskInColumn(status: string, scope: FlowBoardParticipantScope): boolean {
	return scope !== 'other' || status === '요청';
}

export function shouldHideEmptyRequestColumn(scope: FlowBoardParticipantScope): boolean {
	return scope === 'self';
}
