export type WorkspaceRootKind = 'personal' | 'circle' | 'public';

export type WorkspaceRoot = {
	id: string;
	label: string;
	agentPath: string;
	kind: WorkspaceRootKind;
};

export type WorkspaceEntry = {
	name: string;
	agentPath: string;
	isDirectory: boolean;
	size: number;
	modifiedAt: string;
};

export async function fetchWorkspaceRoots(): Promise<WorkspaceRoot[]> {
	const response = await fetch('/files/api/roots', { credentials: 'include' });
	if (!response.ok) throw new Error(await response.text());
	const payload = (await response.json()) as { roots: WorkspaceRoot[] };
	return payload.roots ?? [];
}

export async function listWorkspaceDirectory(path: string): Promise<WorkspaceEntry[]> {
	const response = await fetch(`/files/api/list?path=${encodeURIComponent(path)}`, { credentials: 'include' });
	if (!response.ok) throw new Error(await response.text());
	const payload = (await response.json()) as { entries: WorkspaceEntry[] };
	return payload.entries ?? [];
}

export function workspaceDownloadURL(path: string): string {
	return `/files/api/download?path=${encodeURIComponent(path)}`;
}

export async function uploadWorkspaceFiles(path: string, files: File[]): Promise<string[]> {
	const form = new FormData();
	for (const file of files) form.append('files', file, file.name);
	const response = await fetch(`/files/api/upload?path=${encodeURIComponent(path)}`, {
		method: 'POST',
		credentials: 'include',
		body: form
	});
	if (!response.ok) throw new Error(await response.text());
	const payload = (await response.json()) as { uploaded: string[] };
	return payload.uploaded ?? [];
}
