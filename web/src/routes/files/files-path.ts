import type { WorkspaceRoot } from './files-api';

export type WorkspaceBreadcrumb = {
	label: string;
	path: string;
};

export function workspaceBreadcrumbs(root: WorkspaceRoot, currentPath: string): WorkspaceBreadcrumb[] {
	const breadcrumbs: WorkspaceBreadcrumb[] = [{ label: root.label, path: root.agentPath }];
	if (currentPath === root.agentPath || !currentPath.startsWith(`${root.agentPath}/`)) {
		return breadcrumbs;
	}
	const relativeSegments = currentPath.slice(root.agentPath.length + 1).split('/').filter(Boolean);
	let accumulatedPath = root.agentPath;
	for (const segment of relativeSegments) {
		accumulatedPath = `${accumulatedPath}/${segment}`;
		breadcrumbs.push({ label: segment, path: accumulatedPath });
	}
	return breadcrumbs;
}

export function formatFileSize(bytes: number): string {
	if (bytes < 1024) return `${bytes} B`;
	const units = ['KB', 'MB', 'GB', 'TB'];
	let value = bytes / 1024;
	let unitIndex = 0;
	while (value >= 1024 && unitIndex < units.length - 1) {
		value = value / 1024;
		unitIndex = unitIndex + 1;
	}
	return `${value.toFixed(value >= 10 || Number.isInteger(value) ? 0 : 1)} ${units[unitIndex]}`;
}
