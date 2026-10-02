import { getContext, setContext } from 'svelte';
import {
	fetchWorkspaceRoots,
	listWorkspaceDirectory,
	uploadWorkspaceFiles,
	type WorkspaceEntry,
	type WorkspaceRoot
} from './files-api';
import { workspaceBreadcrumbs, type WorkspaceBreadcrumb } from './files-path';

const filesStateKey = Symbol('files');

export class FilesState {
	roots = $state<WorkspaceRoot[]>([]);
	currentRoot = $state<WorkspaceRoot | null>(null);
	currentPath = $state<string>('');
	selectedFile = $state<WorkspaceEntry | null>(null);
	childrenCache = $state<Record<string, WorkspaceEntry[]>>({});
	loadingPaths = $state<Record<string, boolean>>({});
	isLoading = $state<boolean>(false);
	isUploading = $state<boolean>(false);
	uploadedFraction = $state<number>(0);
	errorMessage = $state<string>('');

	private loadFailedMessage: string;

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
		this.errorMessage = '';
		try {
			this.roots = await fetchWorkspaceRoots();
			if (this.roots.length > 0) await this.openRoot(this.roots[0]);
		} catch (error) {
			this.errorMessage = errorText(error, this.loadFailedMessage);
		}
	}

	async openRoot(root: WorkspaceRoot) {
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

	async loadChildren(path: string) {
		if (path in this.childrenCache || this.loadingPaths[path]) return;
		this.loadingPaths[path] = true;
		try {
			this.childrenCache[path] = await listWorkspaceDirectory(path);
			this.errorMessage = '';
		} catch (error) {
			this.errorMessage = errorText(error, this.loadFailedMessage);
		} finally {
			delete this.loadingPaths[path];
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
		await this.openRoot(this.currentRoot);
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
