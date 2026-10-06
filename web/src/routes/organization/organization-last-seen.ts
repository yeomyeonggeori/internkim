import type { OrgGroup, UserRecord } from '$lib/organization/types';

type LastSeenDirectory = { records: UserRecord[]; groups: OrgGroup[] };

let held: { scope: string; directory: LastSeenDirectory } | null = null;

export function lastSeenDirectory(scope: string): LastSeenDirectory | null {
	return scope && held?.scope === scope ? held.directory : null;
}

export function rememberDirectory(scope: string, directory: LastSeenDirectory): void {
	if (scope) held = { scope, directory };
}

export function forgetLastSeenDirectory(): void {
	held = null;
}
