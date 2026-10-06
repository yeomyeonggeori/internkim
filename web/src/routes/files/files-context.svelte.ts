import { getContext, setContext } from 'svelte';
import {
	fetchWorkspaceRoots,
	listWorkspaceDirectory,
	uploadWorkspaceFiles,
	type WorkspaceEntry,
	type WorkspaceRoot
} from './files-api';
import { workspaceBreadcrumbs, type WorkspaceBreadcrumb } from './files-path';
import { isFilesAccessDenied } from './files-read-error';

const filesStateKey = Symbol('files');

export class FilesState {
	roots = $state<WorkspaceRoot[]>([]);
	currentRoot = $state<WorkspaceRoot | null>(null);
	currentPath = $state<string>('');
	selectedFile = $state<WorkspaceEntry | null>(null);
	childrenCache = $state<Record<string, WorkspaceEntry[]>>({});
	loadingPaths = $state<Record<string, boolean>>({});
	isLoading = $state<boolean>(true);
	isUploading = $state<boolean>(false);
	uploadedFraction = $state<number>(0);
	errorMessage = $state<string>('');

	private loadFailedMessage: string;
	private directoryGeneration = 0;
	private loadingPathGenerations: Record<string, number> = {};

	constructor(loadFailedMessage: string) {
		this.loadFailedMessage = loadFailedMessage;
	}

	get currentEntries(): WorkspaceEntry[] {
		return this.childrenCache[this.currentPath] ?? [];
	}

	get breadcrumbs(): WorkspaceBreadcrumb[] {
		if (!this.currentRoot) return [];
		return workspaceBreadcrumbs(this.currentRoot, this.currentPath);
	}

	async loadRoots() {
		this.isLoading = true;
		this.errorMessage = '';
		try {
			this.roots = await fetchWorkspaceRoots();
			if (this.roots.length > 0) await this.openRoot(this.roots[0]);
		} catch (error) {
			if (isFilesAccessDenied(error)) {
				this.roots = [];
				this.currentRoot = null;
				this.currentPath = '';
				this.childrenCache = {};
				this.selectedFile = null;
			}
			this.errorMessage = errorText(error, this.loadFailedMessage);
		} finally {
			this.isLoading = false;
		}
	}

	async openRoot(root: WorkspaceRoot) {
		this.directoryGeneration += 1;
		this.currentRoot = root;
		this.currentPath = root.agentPath;
		this.selectedFile = null;
		this.childrenCache = {};
		this.isLoading = true;
		try {
			await this.loadChildren(root.agentPath);
		} finally {
			this.isLoading = false;
		}
	}

	async loadChildren(path: string, refresh = false) {
		if ((!refresh && path in this.childrenCache) || (this.loadingPaths[path] && this.loadingPathGenerations[path] === this.directoryGeneration)) return;
		const generation = this.directoryGeneration;
		this.loadingPaths[path] = true;
		this.loadingPathGenerations[path] = generation;
		this.errorMessage = '';
		try {
			const entries = await listWorkspaceDirectory(path);
			if (generation !== this.directoryGeneration) return;
			this.childrenCache[path] = entries;
			if (refresh && path === this.currentPath && this.selectedFile) {
				this.selectedFile = entries.find((entry) => entry.agentPath === this.selectedFile?.agentPath) ?? null;
			}
			this.errorMessage = '';
		} catch (error) {
			if (generation !== this.directoryGeneration) return;
			if (isFilesAccessDenied(error)) {
				delete this.childrenCache[path];
				if (path === this.currentPath) {
					this.directoryGeneration += 1;
					this.childrenCache = {};
					this.selectedFile = null;
				}
			}
			if (path === this.currentPath) this.errorMessage = errorText(error, this.loadFailedMessage);
		} finally {
			if (this.loadingPathGenerations[path] === generation) {
				delete this.loadingPaths[path];
				delete this.loadingPathGenerations[path];
			}
		}
	}

	isLoadingPath(path: string): boolean {
		return this.loadingPaths[path] === true;
	}

	async openDirectory(path: string) {
		this.currentPath = path;
		await this.loadChildren(path);
	}

	selectFile(entry: WorkspaceEntry) {
		this.selectedFile = entry;
	}

	clearSelection() {
		this.selectedFile = null;
	}

	async reload() {
		if (!this.currentRoot) {
			await this.loadRoots();
			return;
		}
		await this.loadChildren(this.currentPath, true);
	}

	async upload(files: File[]) {
		if (!this.currentPath || files.length === 0) return;
		this.isUploading = true;
		this.uploadedFraction = 0;
		this.errorMessage = '';
		try {
			await uploadWorkspaceFiles(this.currentPath, files, (sentBytes, totalBytes) => {
				this.uploadedFraction = totalBytes > 0 ? sentBytes / totalBytes : 0;
			});
			delete this.childrenCache[this.currentPath];
			await this.loadChildren(this.currentPath);
		} catch (error) {
			this.errorMessage = errorText(error, this.loadFailedMessage);
		} finally {
			this.isUploading = false;
		}
	}
}

function errorText(error: unknown, fallback: string): string {
	const message = error instanceof Error ? error.message.trim() : '';
	return message || fallback;
}

export function setFilesState(state: FilesState): FilesState {
	return setContext(filesStateKey, state);
}

export function getFilesState(): FilesState {
	return getContext<FilesState>(filesStateKey);
}
