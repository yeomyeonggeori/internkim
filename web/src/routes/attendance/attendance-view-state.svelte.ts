import { getContext, setContext } from 'svelte';

export type AttendanceWorkspaceView =
	| 'tools'
	| 'status'
	| 'leaveHistory'
	| 'approvals'
	| 'leaveManagement'
	| 'handWritten';

export class AttendanceViewState {
	selected = $state<AttendanceWorkspaceView>('tools');

	constructor(private readonly prepareView: (view: AttendanceWorkspaceView) => Promise<void> = async () => undefined) {}

	prefetch(view: AttendanceWorkspaceView): void {
		void this.prepareView(view);
	}

	select(view: AttendanceWorkspaceView): void {
		this.selected = view;
	}
}

const attendanceViewStateKey = Symbol('attendance-view-state');

export function setAttendanceViewState(state: AttendanceViewState): void {
	setContext(attendanceViewStateKey, state);
}

export function getAttendanceViewState(): AttendanceViewState {
	const state = getContext<AttendanceViewState | undefined>(attendanceViewStateKey);
	if (!state) throw new Error('AttendanceViewState not provided');
	return state;
}
