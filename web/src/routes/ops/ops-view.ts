export type StatusBadgeVariant = 'secondary' | 'outline' | 'destructive';

export function statusBadgeVariant(state: string): StatusBadgeVariant {
	if (state === 'ok' || state === 'succeeded') return 'secondary';
	if (state === 'failed') return 'destructive';
	return 'outline';
}

export function endpointStatusLabel(status?: { state?: string; code?: number }): string {
	if (!status) return 'Unknown';
	if (status.code) return `${formatStatusState(status.state)} ${status.code}`;
	return formatStatusState(status.state);
}

export function shortVersion(value?: string, missingLabel = 'Unchanged'): string {
	const trimmedValue = value?.trim() ?? '';
	if (!trimmedValue) return missingLabel;
	if (trimmedValue.length <= 12) return trimmedValue;
	return trimmedValue.slice(0, 12);
}

export function shortRelease(value?: string): string {
	const trimmedValue = value?.trim() ?? '';
	if (!trimmedValue) return 'No release';
	return shortVersion(trimmedValue);
}

export function modelLabel(status?: { state?: string; model?: string }): string {
	const trimmedValue = status?.model?.trim() ?? '';
	if (!trimmedValue) return 'Not configured';
	return trimmedValue;
}

export function readableModelLabel(status?: { state?: string; model?: string }): string {
	if (status?.model?.trim()) return status.model.trim();
	if (status?.state === 'failed') return 'Unreadable';
	if (status?.state === 'unknown') return 'Unknown';
	return 'Not configured';
}

export function hasVersion(value?: string): boolean {
	return (value ?? '').trim() !== '';
}

export function runtimeVersionDetail(version?: {
	capabilityd?: string;
	blueclawPayload?: string;
	skills?: string;
}): string {
	if (!version) return '';
	return [
		['capabilityd', version.capabilityd],
		['payload', version.blueclawPayload],
		['skills', version.skills]
	]
		.filter(([, value]) => (value ?? '').trim() !== '')
		.map(([label, value]) => `${label}: ${value}`)
		.join('\n');
}

function formatStatusState(state?: string): string {
	if (!state) return 'Unknown';
	if (state === 'ok') return 'OK';
	if (state === 'auth_required') return 'Auth needed';
	if (state === 'not_json') return 'Wrong route';
	return state
		.split('_')
		.map((part) => part.charAt(0).toUpperCase() + part.slice(1))
		.join(' ');
}
