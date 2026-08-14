import type { UpdateAttendanceEventRequest } from '../attendance-api';
import type { AttendanceEvent, AttendanceSummary } from '../attendance-context.svelte';
import {
	fallbackFutureAttendanceLocalTime,
	timeInTimeZone,
	todayDateInTimeZone
} from '../shared/attendance-date';
import { localTimeMinutes } from '../shared/day-timeline';
import { editableAttendanceEventIDs } from './attendance-correction-access';
import type { TeamStatusPersonDaySegment } from './team-status-table-model';

export type WorkEventDraft = {
	eventID: string;
	localDate: string;
	originalLocalTime: string;
	localTime: string;
	originalLocationID: string;
	locationID: string;
};

export type WorkRecordUpdate = {
	eventID: string;
	request: UpdateAttendanceEventRequest;
};

export type WorkRecordEditorDependencies = Readonly<{
	getSummary: () => AttendanceSummary | null;
	hasServerClock: () => boolean;
	getCurrentServerTime: () => Date;
	updateEvents: (updates: WorkRecordUpdate[]) => Promise<void>;
	processingFailedMessage: string;
}>;

export class WorkRecordEditorState {
	isEditing = $state(false);
	drafts = $state<Record<string, WorkEventDraft>>({});
	reason = $state('');
	isSaving = $state(false);
	errorMessage = $state('');
	currentTime = $state(new Date(Number.NaN));
	timeZone = $state('');
	editableEventIDs = $state<Set<string>>(new Set<string>());
	segments = $state<TeamStatusPersonDaySegment[]>([]);

	constructor(private readonly dependencies: WorkRecordEditorDependencies) {}

	get canUse(): boolean {
		return (
			this.dependencies.hasServerClock() &&
			this.dependencies.getSummary()?.timeZoneAuthoritative === true
		);
	}

	get canSave(): boolean {
		return (
			this.canUse &&
			this.hasChanges() &&
			Object.values(this.drafts).every((draft) => draft.localTime !== '') &&
			this.reason.trim() !== '' &&
			!this.isSaving
		);
	}

	canEdit(segments: TeamStatusPersonDaySegment[]): boolean {
		const summary = this.dependencies.getSummary();
		const currentTime = this.currentTime;
		return (
			summary !== null &&
			this.canUse &&
			Number.isFinite(currentTime.getTime()) &&
			editableAttendanceEventIDs(summary, segments, currentTime).size > 0
		);
	}

	open(segments: TeamStatusPersonDaySegment[]): boolean {
		const summary = this.dependencies.getSummary();
		const currentTime = this.dependencies.getCurrentServerTime();
		if (!summary || !this.canUse || !Number.isFinite(currentTime.getTime())) return false;

		this.currentTime = currentTime;
		this.timeZone = summary.timeZone;
		this.segments = segments;
		this.editableEventIDs = editableAttendanceEventIDs(summary, segments, currentTime);
		if (this.editableEventIDs.size === 0) return false;
		this.drafts = createWorkEventDrafts(summary.events, segments, summary.locations[0]?.id ?? '');
		this.reason = '';
		this.errorMessage = '';
		this.isEditing = true;
		return true;
	}

	close(): void {
		if (this.isSaving) return;
		this.reset();
	}

	reset(): void {
		this.isEditing = false;
		this.drafts = {};
		this.reason = '';
		this.errorMessage = '';
		this.currentTime = new Date(Number.NaN);
		this.timeZone = '';
		this.editableEventIDs = new Set<string>();
		this.segments = [];
	}

	setCurrentTime(currentTime: Date): void {
		this.currentTime = currentTime;
		const summary = this.dependencies.getSummary();
		if (summary && this.isEditing) {
			this.editableEventIDs = editableAttendanceEventIDs(summary, this.segments, currentTime);
		}
	}

	matchesTimeZone(timeZone: string | undefined): boolean {
		return this.timeZone !== '' && this.timeZone === timeZone;
	}

	draftFor(eventID: string | undefined): WorkEventDraft | undefined {
		return eventID ? this.drafts[eventID] : undefined;
	}

	isEventEditable(eventID: string | undefined): boolean {
		return eventID !== undefined && this.editableEventIDs.has(eventID);
	}

	isSegmentLocationEditable(segment: TeamStatusPersonDaySegment): boolean {
		if (!this.isEventEditable(segment.startEventID)) return false;
		return segment.endEventID === undefined || this.isEventEditable(segment.endEventID);
	}

	displayTime(
		draft: WorkEventDraft | undefined,
		segmentDate: string,
		fallbackTime: string
	): string {
		if (draft?.localDate !== segmentDate) return fallbackTime;
		return draft.localTime;
	}

	durationMinutes(startTime: string, endTime: string): number {
		return Math.max(0, localTimeMinutes(endTime) - localTimeMinutes(startTime));
	}

	maximumTimeFor(localDate: string | undefined): string | undefined {
		const summary = this.dependencies.getSummary();
		if (!summary || !localDate || !Number.isFinite(this.currentTime.getTime())) return undefined;
		if (localDate !== todayDateInTimeZone(summary.timeZone, this.currentTime)) return undefined;
		return timeInTimeZone(summary.timeZone, this.currentTime);
	}

	updateEventTime(eventID: string | undefined, localTime: string): string {
		const draft = this.draftFor(eventID);
		if (!draft || !this.isEventEditable(eventID)) return localTime;
		const currentTime = this.dependencies.getCurrentServerTime();
		this.currentTime = currentTime;
		draft.localTime = fallbackFutureAttendanceLocalTime(
			draft.localDate,
			localTime,
			this.dependencies.getSummary()?.timeZone,
			currentTime
		);
		return draft.localTime;
	}

	updateSegmentLocation(segment: TeamStatusPersonDaySegment, locationID: string): void {
		if (!this.isSegmentLocationEditable(segment)) return;
		const startDraft = this.draftFor(segment.startEventID);
		if (startDraft) startDraft.locationID = locationID;
		if (segment.endReason !== 'clock_out') return;
		const endDraft = this.draftFor(segment.endEventID);
		if (endDraft) endDraft.locationID = locationID;
	}

	async save(): Promise<void> {
		if (!this.canSave) return;
		this.isSaving = true;
		this.errorMessage = '';
		try {
			await this.dependencies.updateEvents(this.changedUpdates());
			this.reset();
		} catch {
			this.errorMessage = this.dependencies.processingFailedMessage;
		} finally {
			this.isSaving = false;
		}
	}

	private hasChanges(): boolean {
		return Object.values(this.drafts).some(
			(draft) => this.isEventEditable(draft.eventID) && isChangedDraft(draft)
		);
	}

	private changedUpdates(): WorkRecordUpdate[] {
		const reason = this.reason.trim();
		return Object.values(this.drafts)
			.filter((draft) => this.isEventEditable(draft.eventID) && isChangedDraft(draft))
			.map((draft) => ({
				eventID: draft.eventID,
				request: {
					localDate: draft.localDate,
					localTime: draft.localTime,
					locationID: draft.locationID,
					reason
				}
			}));
	}
}

function createWorkEventDrafts(
	events: AttendanceEvent[],
	segments: TeamStatusPersonDaySegment[],
	fallbackLocationID: string
): Record<string, WorkEventDraft> {
	const editableEventIDs = new Set<string>();
	for (const segment of segments) {
		editableEventIDs.add(segment.startEventID);
		if (segment.endEventID) editableEventIDs.add(segment.endEventID);
	}

	const drafts: Record<string, WorkEventDraft> = {};
	for (const event of events) {
		if (!editableEventIDs.has(event.id)) continue;
		const locationID = event.locationID || fallbackLocationID;
		const localTime = event.localTime.slice(0, 5);
		drafts[event.id] = {
			eventID: event.id,
			localDate: event.localDate,
			originalLocalTime: localTime,
			localTime,
			originalLocationID: locationID,
			locationID
		};
	}
	return drafts;
}

function isChangedDraft(draft: WorkEventDraft): boolean {
	return (
		draft.localTime !== draft.originalLocalTime ||
		draft.locationID !== draft.originalLocationID
	);
}
