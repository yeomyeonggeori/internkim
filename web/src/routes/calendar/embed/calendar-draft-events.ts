import { createCalendarModelEvent as createEvent, type CalendarModelEvent as DayFlowEvent } from './calendar-event-model';

const localSortMetadataKey = 'localSortAt';

export type DraftEventParams = Omit<Parameters<typeof createEvent>[0], 'title'> & { title?: string };

export class CalendarDraftEventState {
	readonly pendingCreateEvents = new Map<string, Promise<void>>();
	private readonly draftEventIDs = new Set<string>();
	private readonly draftEventsByID = new Map<string, DayFlowEvent>();
	private readonly draftEventOriginalTitles = new Map<string, string>();
	private readonly draftEventRevisions = new Map<string, number>();
	private readonly eventsDeletedDuringCreate = new Set<string>();

	addCreatedEvent(event: DayFlowEvent): void {
		this.draftEventIDs.add(event.id);
		this.draftEventsByID.set(event.id, event);
		this.draftEventOriginalTitles.set(event.id, (event.title ?? '').trim());
		this.draftEventRevisions.set(event.id, 0);
	}

	isDraftEvent(eventID: string): boolean {
		return this.draftEventIDs.has(eventID);
	}

	removeDraftEvent(eventID: string): void {
		this.draftEventIDs.delete(eventID);
		this.draftEventsByID.delete(eventID);
		this.draftEventOriginalTitles.delete(eventID);
		this.draftEventRevisions.delete(eventID);
	}

	retainDraftEvent(event: DayFlowEvent): void {
		if (!this.isDraftEvent(event.id)) return;
		this.draftEventsByID.set(event.id, event);
		this.draftEventRevisions.set(event.id, (this.draftEventRevisions.get(event.id) ?? 0) + 1);
	}

	draftEvent(eventID: string): DayFlowEvent | undefined {
		return this.draftEventsByID.get(eventID);
	}

	draftEventRevision(eventID: string): number {
		return this.draftEventRevisions.get(eventID) ?? 0;
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
		const existingCommit = this.pendingCreateEvents.get(event.id);
		if (existingCommit) {
			await existingCommit;
			return;
		}
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
		return (title ?? '').trim() === '';
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
