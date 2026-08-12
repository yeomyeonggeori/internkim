import { describe, expect, test } from 'bun:test';
import { refusalToReplaceProduction } from '../../scripts/production-guard';

const upToDate = {
	containsOriginMain: true,
	liveCommit: 'abc1234',
	treeHasLiveCommit: true,
	isReplacingNewerAllowed: false
};

describe('replacing what production is serving', () => {
	test('a tree holding everything production has may deploy', () => {
		expect(refusalToReplaceProduction(upToDate)).toBeNull();
	});

	test('a tree missing the live build is refused', () => {
		const refusal = refusalToReplaceProduction({ ...upToDate, treeHasLiveCommit: false });
		expect(refusal).toContain('abc1234');
		expect(refusal).toContain('--replace-newer');
	});

	test('a tree behind origin/main is refused before anything else', () => {
		const refusal = refusalToReplaceProduction({ ...upToDate, containsOriginMain: false });
		expect(refusal).toContain('origin/main');
	});

	test('putting an older build back is allowed when it is asked for', () => {
		expect(
			refusalToReplaceProduction({ ...upToDate, treeHasLiveCommit: false, isReplacingNewerAllowed: true })
		).toBeNull();
	});

	test('a project with no production deployment yet is not blocked', () => {
		expect(refusalToReplaceProduction({ ...upToDate, liveCommit: null, treeHasLiveCommit: true })).toBeNull();
	});

	test('being behind main still refuses even with --replace-newer', () => {
		expect(
			refusalToReplaceProduction({ ...upToDate, containsOriginMain: false, isReplacingNewerAllowed: true })
		).toContain('origin/main');
	});
});
