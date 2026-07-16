import { createEvent, type Event as DayFlowEvent } from '@dayflow/core';

const legacyKoreanNewEventTitle = '새 일정';
const localSortMetadataKey = 'localSortAt';

export type DraftEventParams = Omit<Parameters<typeof createEvent>[0], 'title'> & { title?: string };

export class CalendarDraftEventState {
	readonly pendingCreateEvents = new Map<string, Promise<void>>();
	private readonly draftEventIDs = new Set<string>();
	private readonly draftEventsByID = new Map<string, DayFlowEvent>();
	private readonly draftEventOriginalTitles = new Map<string, string>();
	private readonly eventsDeletedDuringCreate = new Set<string>();

	constructor(
		private readonly placeholderTitle: () => string,
		private readonly localizedNewEvent: () => string
	) {}

	addCreatedEvent(event: DayFlowEvent): void {
		this.draftEventIDs.add(event.id);
		this.draftEventsByID.set(event.id, event);
		this.draftEventOriginalTitles.set(event.id, (event.title ?? '').trim());
	}

	isDraftEvent(eventID: string): boolean {
		return this.draftEventIDs.has(eventID);
	}

	removeDraftEvent(eventID: string): void {
		this.draftEventIDs.delete(eventID);
		this.draftEventsByID.delete(eventID);
		this.draftEventOriginalTitles.delete(eventID);
	}

	retainDraftEvent(event: DayFlowEvent): void {
		if (!this.isDraftEvent(event.id)) return;
		this.draftEventsByID.set(event.id, event);
	}

	createdEvents(): DayFlowEvent[] {
		return Array.from(this.draftEventsByID.values());
	}

	hasPendingCreate(eventID: string): boolean {
		return this.pendingCreateEvents.has(eventID);
	}

	markDeletedDuringCreate(eventID: string): void {
		this.eventsDeletedDuringCreate.add(eventID);
	}

	wasDeletedDuringCreate(eventID: string): boolean {
		return this.eventsDeletedDuringCreate.has(eventID);
	}

	shouldReportCreateError(eventID: string): boolean {
		return !this.eventsDeletedDuringCreate.has(eventID);
	}

	async trackCreatedEvent(event: DayFlowEvent, createEventOnServer: (event: DayFlowEvent) => Promise<void>): Promise<void> {
		const commitPromise = createEventOnServer(event);
		this.pendingCreateEvents.set(event.id, commitPromise);
		try {
			await commitPromise;
		} finally {
			this.pendingCreateEvents.delete(event.id);
			this.eventsDeletedDuringCreate.delete(event.id);
			this.draftEventOriginalTitles.delete(event.id);
		}
	}

	hasMeaningfulTitle(event: DayFlowEvent): boolean {
		const currentTitle = (event.title ?? '').trim();
		if (this.isPlaceholderTitle(currentTitle)) return false;
		const originalTitle = this.draftEventOriginalTitles.get(event.id);
		if (originalTitle !== undefined && currentTitle === originalTitle) return false;
		return true;
	}

	isPlaceholderTitle(title: string | undefined): boolean {
		return isPlaceholderEventTitle(title, this.placeholderTitle(), this.localizedNewEvent());
	}

	createDraftEvent(params: DraftEventParams): DayFlowEvent {
		return createEvent({
			...params,
			title: '',
			meta: {
				...(params.meta ?? {}),
				[localSortMetadataKey]: new Date().toISOString()
			}
		});
	}

	updateTitleInputPlaceholders(inputPlaceholder: string, commitDraftTitleInput: (input: HTMLInputElement) => void): void {
		const titleInputs = Array.from(
			document.querySelectorAll<HTMLInputElement>(
				'.df-event-detail-panel input[name="title"], .df-mobile-event-drawer input[name="title"]'
			)
		);
		for (const input of titleInputs) {
			const panel = input.closest<HTMLElement>('[data-event-id]');
			const eventID = panel?.dataset.eventId ?? '';
			if (this.isDraftEvent(eventID) && input.dataset.draftTitleCommit !== 'true') {
				input.dataset.draftTitleCommit = 'true';
				input.addEventListener('blur', (event) => {
					if (event.currentTarget instanceof HTMLInputElement) commitDraftTitleInput(event.currentTarget);
				});
				input.addEventListener('keydown', (event) => {
					if (event.currentTarget instanceof HTMLInputElement && event.key === 'Enter') event.currentTarget.blur();
				});
			}
			if (this.isDraftEvent(eventID) && this.isPlaceholderTitle(input.value)) {
				input.value = '';
				input.placeholder = inputPlaceholder;
			} else if (this.isDraftEvent(eventID) && input.value.trim() === '') {
				input.placeholder = inputPlaceholder;
			} else if (input.placeholder === inputPlaceholder) {
				input.removeAttribute('placeholder');
			}
		}
	}

	syncVisibility(events: DayFlowEvent[], stageElement: HTMLElement, eventElementsByID: (eventID: string) => HTMLElement[]): void {
		for (const element of stageElement.querySelectorAll<HTMLElement>('.draft-empty-title-event')) {
			element.classList.remove('draft-empty-title-event');
		}
		for (const event of events) {
			if (!this.isDraftEvent(event.id) || !this.isPlaceholderTitle(event.title)) continue;
			for (const element of eventElementsByID(event.id)) {
				element.classList.add('draft-empty-title-event');
			}
		}
	}
}

function isPlaceholderEventTitle(title: string | undefined, placeholderTitle: string, localizedNewEvent: string): boolean {
	const trimmed = (title ?? '').trim();
	if (trimmed === '') return true;
	if (trimmed === placeholderTitle || trimmed === legacyKoreanNewEventTitle) return true;
	if (trimmed === localizedNewEvent) return true;
	return false;
}
