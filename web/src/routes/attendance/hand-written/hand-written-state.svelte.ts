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
	pageOffset = $state(0);
	totalCount = $state(0);
	selectedTeamKey = $state('');
	selectedChangedByID = $state('');
	appliedDayRange = $state<HandWrittenDayRange>({from:'',to:''});
	private appliedTeamKey = '';
	private appliedChangedByID = '';

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
			const dayRange = this.appliedDayRange.from ? this.appliedDayRange : currentAndPreviousMonth(today);
			const result = await fetchHandWrittenRecords(dayRange, this.pageOffset, this.appliedTeamKey, this.appliedChangedByID);
			if (loadSequence !== this.loadSequence) return;
			if(!this.dayRange.from) this.dayRange = dayRange;
			this.appliedDayRange = dayRange;
			this.records = result.attendance;
			this.totalCount = result.totalCount;
		} catch {
			if (loadSequence !== this.loadSequence) return;
			this.errorMessage = this.text.loadFailed;
		} finally {
			if (loadSequence === this.loadSequence) this.isLoading = false;
		}
	}

	async filter(): Promise<void> { this.appliedDayRange = {...this.dayRange}; this.appliedTeamKey = this.selectedTeamKey; this.appliedChangedByID = this.selectedChangedByID; this.pageOffset = 0; await this.load(); }
	async page(offset: number): Promise<void> { this.pageOffset = Math.max(0, offset); await this.load(); }
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
		this.appliedDayRange = {from:'',to:''};
		this.appliedTeamKey = '';
		this.appliedChangedByID = '';
		this.pageOffset = 0;
		this.totalCount = 0;
		this.selectedTeamKey = '';
		this.selectedChangedByID = '';
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
