export type StatusBadgeVariant = 'secondary' | 'outline' | 'destructive';

export function statusBadgeVariant(state: string): StatusBadgeVariant {
	if (state === 'ok' || state === 'succeeded') return 'secondary';
	if (state === 'failed') return 'destructive';
	return 'outline';
}

export function endpointStatusLabel(status?: { state?: string; code?: number }): string {
	if (!status) return 'unknown';
	if (status.code) return `${status.state} ${status.code}`;
	return status.state ?? 'unknown';
}
