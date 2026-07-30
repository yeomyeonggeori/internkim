import { fetchCalendarEvents } from '../../routes/calendar/embed/calendar-event-persistence';
import { fetchFlowState } from '../../routes/flow/flow-api';
import { participatingEventCount, requestedTaskCount } from '$lib/components/app-badge-counts';

class AppBadgeCounts {
	participatingEvents = $state(0);
	requestedTasks = $state(0);

	load = async (viewerEmail: string) => {
		await Promise.all([this.loadParticipatingEvents(viewerEmail), this.loadRequestedTasks()]);
	};

	private loadParticipatingEvents = async (viewerEmail: string) => {
		const now = new Date();
		const startOfToday = new Date(now.getFullYear(), now.getMonth(), now.getDate());
		const endOfToday = new Date(now.getFullYear(), now.getMonth(), now.getDate() + 1);
		try {
			const events = await fetchCalendarEvents(startOfToday, endOfToday, '');
			this.participatingEvents = participatingEventCount(events, viewerEmail, now);
		} catch {
			this.participatingEvents = 0;
		}
	};

	private loadRequestedTasks = async () => {
		try {
			this.requestedTasks = requestedTaskCount(await fetchFlowState(''));
		} catch {
			this.requestedTasks = 0;
		}
	};
}

export const appBadgeCounts = new AppBadgeCounts();
