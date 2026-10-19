import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { keptBackupManifestSchema } from '../../../src/lib/company/box';

const manifestWrittenByTheBox = new URL('../../../../internal/offsite/testdata/manifest.json', import.meta.url);

describe('a backup the boxes keep', () => {
	test('is read by the web app as the box writes it', () => {
		const parsed = keptBackupManifestSchema.safeParse(JSON.parse(readFileSync(manifestWrittenByTheBox, 'utf8')));
		expect(parsed.success).toBe(true);
	});
});
