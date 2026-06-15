import { describe, expect, test } from 'bun:test';
import { workspaceBreadcrumbs, formatFileSize } from '../../../src/routes/files/files-path';
import { workspaceDownloadURL } from '../../../src/routes/files/files-api';
import type { WorkspaceRoot } from '../../../src/routes/files/files-api';

const engineeringRoot: WorkspaceRoot = {
	id: 'circle:engineering',
	label: 'Engineering',
	agentPath: '/workspace/circles/engineering',
	kind: 'circle'
};

describe('workspaceBreadcrumbs', () => {
	test('returns the root label only when at the root', () => {
		const breadcrumbs = workspaceBreadcrumbs(engineeringRoot, '/workspace/circles/engineering');
		expect(breadcrumbs).toEqual([{ label: 'Engineering', path: '/workspace/circles/engineering' }]);
	});

	test('accumulates a crumb per nested segment', () => {
		const breadcrumbs = workspaceBreadcrumbs(engineeringRoot, '/workspace/circles/engineering/specs/2026');
		expect(breadcrumbs).toEqual([
			{ label: 'Engineering', path: '/workspace/circles/engineering' },
			{ label: 'specs', path: '/workspace/circles/engineering/specs' },
			{ label: '2026', path: '/workspace/circles/engineering/specs/2026' }
		]);
	});

	test('ignores paths outside the root', () => {
		const breadcrumbs = workspaceBreadcrumbs(engineeringRoot, '/workspace/circles/design');
		expect(breadcrumbs).toEqual([{ label: 'Engineering', path: '/workspace/circles/engineering' }]);
	});
});

describe('formatFileSize', () => {
	test('formats bytes, kilobytes, and megabytes', () => {
		expect(formatFileSize(512)).toBe('512 B');
		expect(formatFileSize(2048)).toBe('2 KB');
		expect(formatFileSize(1536)).toBe('1.5 KB');
		expect(formatFileSize(5 * 1024 * 1024)).toBe('5 MB');
	});
});

describe('workspaceDownloadURL', () => {
	test('encodes the agent path', () => {
		expect(workspaceDownloadURL('/workspace/shared/a b.txt')).toBe(
			'/files/api/download?path=%2Fworkspace%2Fshared%2Fa%20b.txt'
		);
	});
});
