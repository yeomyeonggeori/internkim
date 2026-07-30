import type {
	EmployeeLeaveLedgerEntry,
	EmployeeLeaveRequest,
	EmployeeLeaveType
} from './employee-leave-types';

export type LeaveHistoryFilter = 'requests' | 'balance';

export type LeaveHistoryRequestItem = {
	kind: 'request';
	id: string;
	occurredAt: string;
	request: EmployeeLeaveRequest;
	balanceAfterMilliDays?: number;
	isUntracked: boolean;
};

export type LeaveHistoryLedgerItem = {
	kind: 'ledger';
	id: string;
	occurredAt: string;
	entry: EmployeeLeaveLedgerEntry;
	balanceAfterMilliDays: number;
	isUntracked: boolean;
};

export type LeaveHistoryItem = LeaveHistoryRequestItem | LeaveHistoryLedgerItem;

export function buildLeaveHistory(
	requests: EmployeeLeaveRequest[],
	ledgerEntries: EmployeeLeaveLedgerEntry[],
	leaveTypes: EmployeeLeaveType[]
): LeaveHistoryItem[] {
	const typeByID = new Map(leaveTypes.map((leaveType) => [leaveType.id, leaveType]));
	const ledgerByRequestID = ledgerEntries.reduce<Map<string, EmployeeLeaveLedgerEntry[]>>(
		(entriesByRequest, entry) => {
			if (!entry.requestID) return entriesByRequest;
			const entries = entriesByRequest.get(entry.requestID) ?? [];
			entries.push(entry);
			entriesByRequest.set(entry.requestID, entries);
			return entriesByRequest;
		},
		new Map()
	);
	const requestItems: LeaveHistoryRequestItem[] = requests.map((request) => {
		const relatedEntries = [...(ledgerByRequestID.get(request.id) ?? [])].sort((first, second) =>
			first.occurredAt.localeCompare(second.occurredAt)
		);
		const latestRelatedEntry = relatedEntries.at(-1);
		return {
			kind: 'request',
			id: `request:${request.id}`,
			occurredAt: request.updatedAt ?? request.createdAt,
			request,
			balanceAfterMilliDays: latestRelatedEntry?.balanceAfterMilliDays,
			isUntracked: typeByID.get(request.leaveTypeID)?.balanceMode === 'none'
		};
	});
	const ledgerItems: LeaveHistoryLedgerItem[] = ledgerEntries.map((entry) => ({
		kind: 'ledger',
		id: `ledger:${entry.id}`,
		occurredAt: entry.occurredAt,
		entry,
		balanceAfterMilliDays: entry.balanceAfterMilliDays,
		isUntracked: entry.isUntracked
	}));
	return [...requestItems, ...ledgerItems].sort((first, second) =>
		second.occurredAt.localeCompare(first.occurredAt)
	);
}

export function filterLeaveHistory(
	items: LeaveHistoryItem[],
	filter: LeaveHistoryFilter
): LeaveHistoryItem[] {
	if (filter === 'requests') return items.filter((item) => item.kind === 'request');
	return items.filter((item) => item.kind === 'ledger');
}

export function milliDaysValue(milliDays: number): string {
	const days = milliDays / 1000;
	return Number.isInteger(days) ? String(days) : String(Number(days.toFixed(3)));
}
