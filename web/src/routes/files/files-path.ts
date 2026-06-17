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
	const relativeSegments = currentPath
		.slice(root.agentPath.length + 1)
		.split('/')
		.filter(Boolean);
	let accumulatedPath = root.agentPath;
	for (const segment of relativeSegments) {
		accumulatedPath = `${accumulatedPath}/${segment}`;
		breadcrumbs.push({ label: segment, path: accumulatedPath });
	}
	return breadcrumbs;
}
