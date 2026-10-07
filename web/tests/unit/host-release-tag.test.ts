import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { releaseTagSchema } from '../../src/lib/host/host-version';

const versions = JSON.parse(
	readFileSync(new URL('../../../internal/hostversion/testdata/versions.json', import.meta.url), 'utf8')
) as { releaseTags: { accepted: string[]; refused: string[] } };

describe('releaseTagSchema', () => {
	test.each(versions.releaseTags.accepted)('accepts %s as the Go grammar does', (tag) => {
		expect(releaseTagSchema.safeParse(tag).success).toBe(true);
	});

	test.each(versions.releaseTags.refused)('refuses %s as the Go grammar does', (tag) => {
		expect(releaseTagSchema.safeParse(tag).success).toBe(false);
	});
});
