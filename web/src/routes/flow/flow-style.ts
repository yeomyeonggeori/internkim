import { isFlowStatusCompleted, isFlowStatusInProgress, isFlowStatusPaused, isFlowStatusPlanned, isFlowStatusRejected, isFlowStatusRequested, isFlowStatusStopped } from './flow-status';

export function statusBadgeClass(status: string): string {
	if (isFlowStatusCompleted(status)) return 'bg-[#d4edbc] text-[#1f3826] border-transparent';
	if (isFlowStatusInProgress(status)) return 'bg-[#bfe1f6] text-[#0b3d63] border-transparent';
	if (isFlowStatusPlanned(status)) return 'bg-[#ffe5a0] text-[#473821] border-transparent';
	if (isFlowStatusRequested(status)) return 'bg-[#e6cff2] text-[#3d1c52] border-transparent';
	if (isFlowStatusPaused(status)) return 'bg-[#ffcfc9] text-[#5b1c14] border-transparent';
	if (isFlowStatusRejected(status) || isFlowStatusStopped(status)) return 'bg-[#f6c1bd] text-[#5b1c14] border-transparent';
	return 'bg-muted text-muted-foreground border-transparent';
}

export function statusOutlineClass(status: string): string {
	if (isFlowStatusCompleted(status)) return 'border-[#a8d08d] bg-[#d4edbc]/35 text-[#1f3826]';
	if (isFlowStatusInProgress(status)) return 'border-[#8ec7e8] bg-[#bfe1f6]/35 text-[#0b3d63]';
	if (isFlowStatusPlanned(status)) return 'border-[#e8c96f] bg-[#ffe5a0]/35 text-[#473821]';
	if (isFlowStatusRequested(status)) return 'border-[#c6a3d9] bg-[#e6cff2]/35 text-[#3d1c52]';
	if (isFlowStatusPaused(status)) return 'border-[#e6aaa4] bg-[#ffcfc9]/35 text-[#5b1c14]';
	if (isFlowStatusRejected(status) || isFlowStatusStopped(status)) return 'border-[#d99a95] bg-[#f6c1bd]/35 text-[#5b1c14]';
	return 'border-border bg-muted/35 text-muted-foreground';
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
