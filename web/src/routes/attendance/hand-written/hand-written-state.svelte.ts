import { getContext, setContext } from 'svelte';
import type { AttendanceText } from '../text';
import {
	currentAndPreviousMonth,
	fetchHandWrittenRecords,
	undoHandWrittenRecord,
	type HandWrittenDayRange,
	type HandWrittenRecord
} from './hand-written-records';

export class HandWrittenState {
	records = $state<HandWrittenRecord[]>([]);
	dayRange = $state<HandWrittenDayRange>({ from: '', to: '' });
	isLoading = $state(false);
	undoingEventID = $state('');
	errorMessage = $state('');
	private loadSequence = 0;
	private today = '';

	constructor(
		private readonly text: AttendanceText['handWritten'],
		private readonly onUndone: () => Promise<void> = async () => undefined
	) {}

	async load(today = this.today): Promise<void> {
		if (!today) return;
		this.today = today;
		const loadSequence = ++this.loadSequence;
		this.isLoading = true;
		this.errorMessage = '';
		try {
			const dayRange = currentAndPreviousMonth(today);
			const records = await fetchHandWrittenRecords(dayRange);
			if (loadSequence !== this.loadSequence) return;
			this.dayRange = dayRange;
			this.records = records;
		} catch {
			if (loadSequence !== this.loadSequence) return;
			this.errorMessage = this.text.loadFailed;
		} finally {
			if (loadSequence === this.loadSequence) this.isLoading = false;
		}
	}

	async undo(record: HandWrittenRecord, reason: string): Promise<void> {
		if (this.undoingEventID) return;
		this.undoingEventID = record.eventID;
		this.errorMessage = '';
		try {
			await undoHandWrittenRecord(record, reason);
			await this.load();
			await this.onUndone();
		} catch {
			this.errorMessage = this.text.undoFailed;
		} finally {
			this.undoingEventID = '';
		}
	}

	clear(): void {
		this.loadSequence += 1;
		this.records = [];
		this.dayRange = { from: '', to: '' };
		this.isLoading = false;
		this.errorMessage = '';
	}
}

const handWrittenStateKey = Symbol('hand-written-state');

export function setHandWrittenState(state: HandWrittenState): void {
	setContext(handWrittenStateKey, state);
}

export function getHandWrittenState(): HandWrittenState {
	const state = getContext<HandWrittenState | undefined>(handWrittenStateKey);
	if (!state) throw new Error('HandWrittenState not provided');
	return state;
}
