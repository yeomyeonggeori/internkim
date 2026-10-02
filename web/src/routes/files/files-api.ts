import { callCompanyApp } from '$lib/host-bridge';
import { isSupabaseConfigured } from '$lib/supabase';
import {
	copiedForReading,
	signedForReading,
	uploadedThroughTheHost,
	type TransferProgress
} from '$lib/transfer/company-transfer';

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

async function failedResponseMessage(response: Response): Promise<string> {
	const body = (await response.text()).trim();
	if (!body || body.startsWith('<') || body.length > 200) {
		return `${response.status} ${response.statusText}`.trim();
	}
	return body;
}

export async function fetchWorkspaceRoots(): Promise<WorkspaceRoot[]> {
	if (isSupabaseConfigured()) {
		const answered = await askTheCompanyApp('person.files.roots', {});
		return (answered as { roots?: WorkspaceRoot[] }).roots ?? [];
	}
	const response = await fetch('/files/api/roots', { credentials: 'include' });
	if (!response.ok) throw new Error(await failedResponseMessage(response));
	const payload = (await response.json()) as { roots: WorkspaceRoot[] };
	return payload.roots ?? [];
}

export async function listWorkspaceDirectory(path: string): Promise<WorkspaceEntry[]> {
	if (isSupabaseConfigured()) {
		const answered = await askTheCompanyApp('person.files.list', { path });
		return (answered as { entries?: WorkspaceEntry[] }).entries ?? [];
	}
	const response = await fetch(`/files/api/list?path=${encodeURIComponent(path)}`, { credentials: 'include' });
	if (!response.ok) throw new Error(await failedResponseMessage(response));
	const payload = (await response.json()) as { entries: WorkspaceEntry[] };
	return payload.entries ?? [];
}

export function workspaceDownloadURL(path: string): string {
	return `/files/api/download?path=${encodeURIComponent(path)}`;
}

export function isWorkspaceOnTheCompanyComputer(): boolean {
	return isSupabaseConfigured();
}

export async function workspaceFileForReading(
	entry: WorkspaceEntry,
	purpose: 'preview' | 'download',
	onProgress?: TransferProgress
): Promise<string> {
	const copy = await copiedForReading('person.files.download', { path: entry.agentPath }, onProgress);
	return signedForReading(copy.address, purpose === 'download' ? entry.name : undefined);
}

export type UploadProgress = (sentBytes: number, totalBytes: number) => void;

async function uploadThroughTheCompanyComputer(path: string, files: File[], onProgress: UploadProgress): Promise<string[]> {
	const totalBytes = files.reduce((sum, file) => sum + file.size, 0);
	let finishedBytes = 0;
	const uploaded: string[] = [];
	for (const file of files) {
		await uploadedThroughTheHost(
			'person.files.upload',
			file,
			file.type || 'application/octet-stream',
			{ directoryPath: path, filename: file.name },
			(sentBytes) => onProgress(finishedBytes + sentBytes, totalBytes)
		);
		finishedBytes += file.size;
		uploaded.push(file.name);
	}
	return uploaded;
}

export async function uploadWorkspaceFiles(
	path: string,
	files: File[],
	onProgress: UploadProgress = () => undefined
): Promise<string[]> {
	if (isSupabaseConfigured()) return uploadThroughTheCompanyComputer(path, files, onProgress);
	const form = new FormData();
	for (const file of files) form.append('files', file, file.name);
	const response = await fetch(`/files/api/upload?path=${encodeURIComponent(path)}`, {
		method: 'POST',
		credentials: 'include',
		body: form
	});
	if (!response.ok) throw new Error(await failedResponseMessage(response));
	const payload = (await response.json()) as { uploaded: string[] };
	return payload.uploaded ?? [];
}

async function askTheCompanyApp(capability: string, body: Record<string, unknown>): Promise<unknown> {
	const answer = await callCompanyApp({ capability, body });
	if (answer.status >= 400) throw new Error(companyAppErrorMessage(answer.body, answer.status));
	return answer.body;
}

function companyAppErrorMessage(body: unknown, status: number): string {
	if (typeof body === 'object' && body !== null && typeof (body as { error?: unknown }).error === 'string') {
		return (body as { error: string }).error;
	}
	return `the company app answered ${status}`;
}
