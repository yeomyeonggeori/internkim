import { getContext, setContext } from 'svelte';

export type AttendanceWorkspaceView =
	| 'tools'
	| 'status'
	| 'leaveHistory'
	| 'approvals'
	| 'leaveManagement';

export class AttendanceViewState {
	selected = $state<AttendanceWorkspaceView>('tools');

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
