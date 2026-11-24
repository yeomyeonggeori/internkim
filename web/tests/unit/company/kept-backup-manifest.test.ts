import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { keptBackupManifestSchema, keptBaseBackupSchema } from '../../../src/lib/company/box';

const manifestWrittenByTheBox = new URL('../../../../internal/offsite/testdata/manifest.json', import.meta.url);
const baseBackupWrittenByTheBox = new URL('../../../../internal/walstream/testdata/base-backup.json', import.meta.url);

describe('a backup the boxes keep', () => {
	test('is read by the web app as the box writes it', () => {
		const parsed = keptBackupManifestSchema.safeParse(JSON.parse(readFileSync(manifestWrittenByTheBox, 'utf8')));
		expect(parsed.success).toBe(true);
	});

	test('starts the log with a base backup the web app reads as the box writes it', () => {
		const parsed = keptBaseBackupSchema.safeParse(JSON.parse(readFileSync(baseBackupWrittenByTheBox, 'utf8')));
		expect(parsed.success).toBe(true);
	});
});
