export const flowStatus = {
	requested: '요청',
	planned: '예정',
	inProgress: '진행',
	completed: '완료',
	paused: '일시정지',
	rejected: '기각',
	stopped: '중단'
} as const;

export type FlowStatus = (typeof flowStatus)[keyof typeof flowStatus];

export function cleanFlowStatus(status: string): string {
	return status.trim();
}

export function isFlowStatusCompleted(status: string): boolean {
	return cleanFlowStatus(status) === flowStatus.completed;
}

export function isFlowStatusInProgress(status: string): boolean {
	return cleanFlowStatus(status) === flowStatus.inProgress;
}

export function isFlowStatusPlanned(status: string): boolean {
	return cleanFlowStatus(status) === flowStatus.planned;
}

export function isFlowStatusRequested(status: string): boolean {
	return cleanFlowStatus(status) === flowStatus.requested;
}

export function isFlowStatusPaused(status: string): boolean {
	return cleanFlowStatus(status) === flowStatus.paused;
}

export function isFlowStatusRejected(status: string): boolean {
	return cleanFlowStatus(status) === flowStatus.rejected;
}

export function isFlowStatusStopped(status: string): boolean {
	return cleanFlowStatus(status) === flowStatus.stopped;
}
