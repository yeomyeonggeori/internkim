import { isFlowStatusRequested } from '../../routes/flow/flow-status';
import { currentFlowMember } from '../../routes/flow/flow-task-workspace-model';
import type { FlowState } from '../../routes/flow/flow-types';
import type { CalendarEvent } from '../../routes/calendar/embed/calendar-event-persistence';

export function participatingEventCount(events: CalendarEvent[], viewerEmail: string, now: Date): number {
	const normalizedViewerEmail = viewerEmail.trim().toLowerCase();
	if (!normalizedViewerEmail) return 0;
	return events.filter((event) => hasNotEnded(event, now) && includesViewer(event, normalizedViewerEmail)).length;
}

function hasNotEnded(event: CalendarEvent, now: Date): boolean {
	const endTime = new Date(event.endISO).getTime();
	if (Number.isNaN(endTime)) return true;
	return endTime > now.getTime();
}

function includesViewer(event: CalendarEvent, normalizedViewerEmail: string): boolean {
	return (event.participants ?? []).some(
		(participant) => (participant.email ?? '').trim().toLowerCase() === normalizedViewerEmail
	);
}

export function requestedTaskCount(state: FlowState): number {
	const viewer = currentFlowMember(state);
	if (!viewer) return 0;
	return state.tasks.filter(
		(task) =>
			isFlowStatusRequested(task.status) &&
			(task.ownerID === viewer.id || task.participantIDs.includes(viewer.id))
	).length;
}
