import type { OrgGroup, UserRecord } from '$lib/organization/types';

type LastSeenDirectory = { records: UserRecord[]; groups: OrgGroup[] };

let held: LastSeenDirectory | null = null;

export function lastSeenDirectory(): LastSeenDirectory | null {
	return held;
}

export function rememberDirectory(directory: LastSeenDirectory): void {
	held = directory;
}

export function forgetLastSeenDirectory(): void {
	held = null;
}
