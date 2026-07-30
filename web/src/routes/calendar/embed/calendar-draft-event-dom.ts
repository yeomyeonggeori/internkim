import type { CalendarModelEvent as DayFlowEvent } from './calendar-event-model';
import { CalendarDraftEventState } from './calendar-draft-events';
import { calendarEventElementsByID } from './calendar-event-elements';
import type { CalendarProgrammaticUpdateState } from './calendar-programmatic-updates';

type CalendarDraftEventDOMContext = {
	isBrowser: () => boolean;
	getStageElement: () => HTMLElement | null;
	getCalendarEvents: () => DayFlowEvent[];
	updateCalendarEvent: (eventID: string, changes: Partial<DayFlowEvent>, shouldRender: boolean) => Promise<void>;
	openEventDetails: (eventID: string) => void;
	resetDraftEventTitle: (eventID: string) => Promise<void>;
	persistCreatedEvent: (event: DayFlowEvent) => Promise<void>;
	text: {
		draftTitlePlaceholder: string;
	};
};

export type CalendarDraftEventDOMActions = {
	openEventDetailsAfterRender: (eventID: string) => void;
	scheduleDraftTitleInputPlaceholderUpdates: () => void;
	scheduleDraftEventVisibilitySync: () => void;
};

export function createCalendarDraftEventDOMActions(
	context: CalendarDraftEventDOMContext,
	draftEvents: CalendarDraftEventState,
	programmaticUpdates: CalendarProgrammaticUpdateState
): CalendarDraftEventDOMActions {
	function openEventDetailsAfterRender(eventID: string): void {
		if (!context.isBrowser()) return;
		requestAnimationFrame(() => {
			requestAnimationFrame(() => {
				context.openEventDetails(eventID);
				scheduleDraftTitleInputPlaceholderUpdates();
			});
		});
	}

	function scheduleDraftTitleInputPlaceholderUpdates(): void {
		if (!context.isBrowser()) return;
		updateDraftTitleInputPlaceholders();
		requestAnimationFrame(updateDraftTitleInputPlaceholders);
		window.setTimeout(updateDraftTitleInputPlaceholders, 50);
	}

	function updateDraftTitleInputPlaceholders(): void {
		if (!context.isBrowser()) return;
		draftEvents.updateTitleInputPlaceholders(context.text.draftTitlePlaceholder, (input) => {
			void commitDraftTitleInput(input);
		});
	}

	async function commitDraftTitleInput(input: HTMLInputElement): Promise<void> {
		const panel = input.closest<HTMLElement>('[data-event-id]');
		const eventID = panel?.dataset.eventId ?? '';
		if (!draftEvents.isDraftEvent(eventID)) return;
		const title = input.value.trim();
		if (draftEvents.isPlaceholderTitle(title)) {
			await context.resetDraftEventTitle(eventID);
			return;
		}
		await programmaticUpdates.run(eventID, () => context.updateCalendarEvent(eventID, { title }, false));
		const event = context.getCalendarEvents().find((candidate) => candidate.id === eventID);
		if (!event || !draftEvents.hasMeaningfulTitle(event)) return;
		syncDraftEventVisibility();
		await context.persistCreatedEvent(event);
	}

	function scheduleDraftEventVisibilitySync(): void {
		if (!context.isBrowser()) return;
		syncDraftEventVisibility();
		requestAnimationFrame(syncDraftEventVisibility);
		window.setTimeout(syncDraftEventVisibility, 50);
	}

	function syncDraftEventVisibility(): void {
		const stageElement = context.getStageElement();
		if (!stageElement) return;
		draftEvents.syncVisibility(context.getCalendarEvents(), stageElement, (eventID) =>
			calendarEventElementsByID(stageElement, eventID)
		);
	}

	return {
		openEventDetailsAfterRender,
		scheduleDraftTitleInputPlaceholderUpdates,
		scheduleDraftEventVisibilitySync
	};
}
