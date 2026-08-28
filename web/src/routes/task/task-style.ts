import { isTaskStatusCompleted, isTaskStatusInProgress, isTaskStatusPaused, isTaskStatusPlanned, isTaskStatusRejected, isTaskStatusRequested, isTaskStatusStopped } from './task-status';

export function statusBadgeClass(status: string): string {
	if (isTaskStatusCompleted(status)) return 'bg-[#d4edbc] text-[#1f3826] border-transparent';
	if (isTaskStatusInProgress(status)) return 'bg-[#bfe1f6] text-[#0b3d63] border-transparent';
	if (isTaskStatusPlanned(status)) return 'bg-[#ffe5a0] text-[#473821] border-transparent';
	if (isTaskStatusRequested(status)) return 'bg-[#e6cff2] text-[#3d1c52] border-transparent';
	if (isTaskStatusPaused(status)) return 'bg-[#ffcfc9] text-[#5b1c14] border-transparent';
	if (isTaskStatusRejected(status) || isTaskStatusStopped(status)) return 'bg-[#f6c1bd] text-[#5b1c14] border-transparent';
	return 'bg-muted text-muted-foreground border-transparent';
}

export function statusIconClass(status: string): string {
	if (isTaskStatusCompleted(status)) return 'text-[#16a34a]';
	if (isTaskStatusInProgress(status)) return 'text-[#0284c7]';
	if (isTaskStatusPlanned(status)) return 'text-[#d97706]';
	if (isTaskStatusRequested(status)) return 'text-[#7c3aed]';
	if (isTaskStatusPaused(status)) return 'text-[#e11d48]';
	if (isTaskStatusRejected(status) || isTaskStatusStopped(status)) return 'text-[#dc2626]';
	return 'text-muted-foreground';
}

export function relationshipStatusIconClass(status: string): string {
	if (isTaskStatusCompleted(status)) return 'text-[#16a34a]';
	return 'text-[#7c3aed]';
}

export function sizeBadgeClass(size: string): string {
	const baseClass = 'rounded-md border border-transparent font-mono tabular-nums text-white shadow-none';
	switch (size) {
		case 'XS':
			return `${baseClass} bg-[#6b7280]`;
		case 'S':
			return `${baseClass} bg-[#2563eb]`;
		case 'M':
			return `${baseClass} bg-[#16a34a]`;
		case 'L':
			return `${baseClass} bg-[#d97706]`;
		case 'XL':
			return `${baseClass} bg-[#dc2626]`;
		case 'XXL':
			return `${baseClass} bg-[#991b1b]`;
		default:
			return `${baseClass} bg-muted-foreground`;
	}
}

export function compareOptionalDate(left: string | undefined, right: string | undefined): number {
	const leftValue = left ?? '';
	const rightValue = right ?? '';
	if (leftValue === rightValue) return 0;
	if (!leftValue) return 1;
	if (!rightValue) return -1;
	return leftValue < rightValue ? -1 : 1;
}
