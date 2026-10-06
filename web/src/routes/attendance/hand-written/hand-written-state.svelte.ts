import { getContext, setContext } from 'svelte';
import { ToolRefused } from '$lib/public-api-call';
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
	private today = $state('');
	pageOffset = $state(0);
	totalCount = $state(0);
	selectedTeamKey = $state('');
	selectedChangedByID = $state('');
	appliedDayRange = $state<HandWrittenDayRange>({from:'',to:''});
	private appliedTeamKey = $state('');
	private appliedChangedByID = $state('');

	get hasAppliedFilters(): boolean {
		if (this.appliedTeamKey || this.appliedChangedByID) return true;
		if (!this.today || !this.appliedDayRange.from) return false;
		const defaultRange = currentAndPreviousMonth(this.today);
		return this.appliedDayRange.from !== defaultRange.from || this.appliedDayRange.to !== defaultRange.to;
	}

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
		} catch (error) {
			if (loadSequence !== this.loadSequence) return;
			if (error instanceof ToolRefused && [401, 403].includes(error.status)) { this.records = []; this.totalCount = 0; }
			this.errorMessage = this.text.loadFailed;
		} finally {
			if (loadSequence === this.loadSequence) this.isLoading = false;
		}
	}

	async filter(): Promise<void> {
		if (this.appliedDayRange.from !== this.dayRange.from || this.appliedDayRange.to !== this.dayRange.to || this.appliedTeamKey !== this.selectedTeamKey || this.appliedChangedByID !== this.selectedChangedByID || this.pageOffset !== 0) this.records = [];
		this.appliedDayRange = { ...this.dayRange };
		this.appliedTeamKey = this.selectedTeamKey;
		this.appliedChangedByID = this.selectedChangedByID;
		this.pageOffset = 0;
		await this.load();
	}
	async page(offset: number): Promise<void> {
		const nextOffset = Math.max(0, offset);
		if (this.pageOffset !== nextOffset) this.records = [];
		this.pageOffset = nextOffset;
		await this.load();
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
