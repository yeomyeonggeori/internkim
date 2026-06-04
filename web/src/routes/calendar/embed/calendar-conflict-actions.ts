import {
	dismissCalendarConflictOnServer as dismissCalendarConflictOnServerDefault,
	dismissCalendarConflictsOnServer as dismissCalendarConflictsOnServerDefault,
	fetchCalendarConflicts as fetchCalendarConflictsDefault,
	type CalendarConflict
} from './calendar-conflicts';

export type CalendarConflictActionContext = {
	isBrowser: () => boolean;
	errorFallback: () => string;
	getCalendarConflicts: () => CalendarConflict[];
	setCalendarConflicts: (conflicts: CalendarConflict[]) => void;
	setErrorMessage: (message: string) => void;
	syncRemoteCalendarAndRefresh: () => Promise<void>;
};

type CalendarConflictActionDependencies = {
	fetchCalendarConflicts?: () => Promise<CalendarConflict[]>;
	dismissCalendarConflictOnServer?: (conflictID: number) => Promise<void>;
	dismissCalendarConflictsOnServer?: (conflictIDs: number[]) => Promise<void>;
};

export type CalendarConflictActions = {
	loadCalendarConflicts: () => Promise<void>;
	dismissCalendarConflict: (conflictID: number) => Promise<void>;
	dismissAllConflictsAndRefresh: () => Promise<void>;
};

export function createCalendarConflictActions(
	context: CalendarConflictActionContext,
	dependencies: CalendarConflictActionDependencies = {}
): CalendarConflictActions {
	const fetchCalendarConflicts = dependencies.fetchCalendarConflicts ?? fetchCalendarConflictsDefault;
	const dismissCalendarConflictOnServer =
		dependencies.dismissCalendarConflictOnServer ?? dismissCalendarConflictOnServerDefault;
	const dismissCalendarConflictsOnServer =
		dependencies.dismissCalendarConflictsOnServer ?? dismissCalendarConflictsOnServerDefault;

	async function loadCalendarConflicts(): Promise<void> {
		if (!context.isBrowser()) return;
		try {
			context.setCalendarConflicts(await fetchCalendarConflicts());
		} catch {
			return;
		}
	}

	async function dismissCalendarConflict(conflictID: number): Promise<void> {
		try {
			await dismissCalendarConflictOnServer(conflictID);
			context.setCalendarConflicts(context.getCalendarConflicts().filter((conflict) => conflict.id !== conflictID));
		} catch (error) {
			context.setErrorMessage(error instanceof Error ? error.message : context.errorFallback());
		}
	}

	async function dismissAllConflictsAndRefresh(): Promise<void> {
		const pendingIDs = context.getCalendarConflicts().map((conflict) => conflict.id);
		try {
			await dismissCalendarConflictsOnServer(pendingIDs);
			context.setCalendarConflicts([]);
			await context.syncRemoteCalendarAndRefresh();
		} catch (error) {
			context.setErrorMessage(error instanceof Error ? error.message : context.errorFallback());
			await loadCalendarConflicts();
		}
	}

	return {
		loadCalendarConflicts,
		dismissCalendarConflict,
		dismissAllConflictsAndRefresh
	};
}
