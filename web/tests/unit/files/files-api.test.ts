import { describe, expect, test } from 'bun:test';
import { workspaceDownloadURL } from '../../../src/routes/files/files-api';

describe('workspaceDownloadURL', () => {
	test('encodes the agent path', () => {
		expect(workspaceDownloadURL('/workspace/shared/a b.txt')).toBe(
			'/files/api/download?path=%2Fworkspace%2Fshared%2Fa%20b.txt'
		);
	});
});
