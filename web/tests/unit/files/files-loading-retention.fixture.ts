import { beforeEach, expect, mock, test } from 'bun:test';
import type { WorkspaceEntry, WorkspaceRoot } from '../../../src/routes/files/files-api';
import { FilesReadError } from '../../../src/routes/files/files-read-error';

Reflect.set(globalThis, '$state', <Value>(value: Value): Value => value);
const root: WorkspaceRoot = { id: 'personal', label: 'Personal', agentPath: '/workspace/private/sample', kind: 'personal' };
const entry: WorkspaceEntry = { name: 'sample.md', agentPath: `${root.agentPath}/sample.md`, isDirectory: false, size: 42, modifiedAt: '2026-10-06T03:00:00Z' };
let roots: () => Promise<WorkspaceRoot[]>;
let entries: (path: string) => Promise<WorkspaceEntry[]>;
mock.module('../../../src/routes/files/files-api', () => ({ fetchWorkspaceRoots: () => roots(), listWorkspaceDirectory: (path: string) => entries(path), uploadWorkspaceFiles: async () => {} }));
const { FilesState } = await import('../../../src/routes/files/files-context.svelte');

beforeEach(() => { roots = async () => [root]; entries = async () => [entry]; });

test('the first roots read is pending before mount and until directory contents arrive', async () => {
	const state = new FilesState('Cannot load files');
	expect(state.isLoading).toBe(true);
	const rootsGate = Promise.withResolvers<WorkspaceRoot[]>();
	const entriesGate = Promise.withResolvers<WorkspaceEntry[]>();
	roots = () => rootsGate.promise;
	entries = () => entriesGate.promise;
	const loading = state.loadRoots();
	expect(state.isLoading).toBe(true);
	rootsGate.resolve([root]);
	await Promise.resolve();
	expect(state.isLoading).toBe(true);
	entriesGate.resolve([entry]);
	await loading;
	expect(state.isLoading).toBe(false);
	expect(state.currentEntries).toEqual([entry]);
});

test('refresh retains the current directory and selected file while waiting', async () => {
	const state = new FilesState('Cannot load files');
	await state.loadRoots();
	await state.openDirectory(`${root.agentPath}/nested`);
	state.selectFile(entry);
	const gate = Promise.withResolvers<WorkspaceEntry[]>();
	const reads: string[] = [];
	entries = (path) => { reads.push(path); return gate.promise; };
	const refresh = state.reload();
	expect(reads).toEqual([`${root.agentPath}/nested`]);
	expect(state.currentPath).toBe(`${root.agentPath}/nested`);
	expect(state.currentEntries).toEqual([entry]);
	expect(state.selectedFile).toBe(entry);
	expect(state.isLoadingPath(state.currentPath)).toBe(true);
	gate.resolve([{ ...entry, name: 'updated.md' }]);
	await refresh;
	expect(state.currentEntries[0].name).toBe('updated.md');
	expect(state.selectedFile?.name).toBe('updated.md');
	expect(state.isLoadingPath(state.currentPath)).toBe(false);
});

test('a failed refresh retains readable content and reports the error', async () => {
	const state = new FilesState('Cannot load files');
	await state.loadRoots();
	entries = async () => { throw new Error('temporarily offline'); };
	await state.reload();
	expect(state.currentEntries).toEqual([entry]);
	expect(state.errorMessage).toBe('temporarily offline');
	expect(state.isLoadingPath(state.currentPath)).toBe(false);
});

test('a successful empty roots read settles while a failed read remains distinguishable', async () => {
	roots = async () => [];
	const state = new FilesState('Cannot load files');
	await state.loadRoots();
	expect(state.isLoading).toBe(false);
	expect(state.errorMessage).toBe('');
	roots = async () => { throw new Error('roots unavailable'); };
	await state.loadRoots();
	expect(state.isLoading).toBe(false);
	expect(state.errorMessage).toBe('roots unavailable');
});

for (const status of [401, 403]) {
	test(`a denied ${status} directory refresh removes cached entries and the selected preview`, async () => {
		const state = new FilesState('Cannot load files');
		await state.loadRoots();
		state.selectFile(entry);
		entries = async () => { throw new FilesReadError('Access refused', status); };
		await state.reload();
		expect(state.currentEntries).toEqual([]);
		expect(state.childrenCache).toEqual({});
		expect(state.selectedFile).toBeNull();
		expect(state.errorMessage).toBe('Access refused');
	});
}

test('a successful refresh closes a preview when its file was removed', async () => {
	const state = new FilesState('Cannot load files');
	await state.loadRoots();
	state.selectFile(entry);
	entries = async () => [];
	await state.reload();
	expect(state.selectedFile).toBeNull();
});

test('an old directory request cannot overwrite a newly selected root', async () => {
	const state = new FilesState('Cannot load files');
	await state.loadRoots();
	const gate = Promise.withResolvers<WorkspaceEntry[]>();
	entries = () => gate.promise;
	const oldRead = state.reload();
	entries = async () => [{ ...entry, name: 'new-root.md', agentPath: '/workspace/shared/new-root.md' }];
	await state.openRoot({ id: 'public', label: 'Public', kind: 'public', agentPath: '/workspace/shared' });
	gate.reject(new FilesReadError('Old root denied', 403));
	await oldRead;
	expect(state.currentEntries[0].name).toBe('new-root.md');
	expect(state.errorMessage).toBe('');
});

test('A to B to A starts a fresh A read without an older pending A request suppressing it', async () => {
	const state = new FilesState('Cannot load files');
	await state.loadRoots();
	const oldGate = Promise.withResolvers<WorkspaceEntry[]>();
	entries = () => oldGate.promise;
	const oldRead = state.reload();
	entries = async () => [{ ...entry, agentPath: '/workspace/shared/public.md', name: 'public.md' }];
	await state.openRoot({ id: 'public', label: 'Public', kind: 'public', agentPath: '/workspace/shared' });
	const currentGate = Promise.withResolvers<WorkspaceEntry[]>();
	let currentReads = 0;
	entries = () => { currentReads += 1; return currentGate.promise; };
	const returning = state.openRoot(root);
	expect(currentReads).toBe(1);
	oldGate.resolve([{ ...entry, name: 'outdated.md' }]);
	await oldRead;
	expect(state.isLoadingPath(root.agentPath)).toBe(true);
	currentGate.resolve([{ ...entry, name: 'current.md' }]);
	await returning;
	expect(state.currentEntries[0].name).toBe('current.md');
	expect(state.isLoadingPath(root.agentPath)).toBe(false);
});
