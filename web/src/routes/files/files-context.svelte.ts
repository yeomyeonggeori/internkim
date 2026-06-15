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
	entries = $state<WorkspaceEntry[]>([]);
	isLoading = $state<boolean>(false);
	isUploading = $state<boolean>(false);
	errorMessage = $state<string>('');

	private loadFailedMessage: string;

	constructor(loadFailedMessage: string) {
		this.loadFailedMessage = loadFailedMessage;
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
		await this.navigateTo(root.agentPath);
	}

	async navigateTo(path: string) {
		this.currentPath = path;
		await this.refresh();
	}

	async reload() {
		if (this.roots.length === 0) {
			await this.loadRoots();
			return;
		}
		await this.refresh();
	}

	async refresh() {
		if (!this.currentPath) return;
		this.isLoading = true;
		this.errorMessage = '';
		try {
			this.entries = await listWorkspaceDirectory(this.currentPath);
		} catch (error) {
			this.entries = [];
			this.errorMessage = errorText(error, this.loadFailedMessage);
		} finally {
			this.isLoading = false;
		}
	}

	async upload(files: File[]) {
		if (!this.currentPath || files.length === 0) return;
		this.isUploading = true;
		this.errorMessage = '';
		try {
			await uploadWorkspaceFiles(this.currentPath, files);
			await this.refresh();
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
