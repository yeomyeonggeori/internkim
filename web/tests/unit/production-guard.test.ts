import { describe, expect, test } from 'bun:test';
import { mainCommitOfLiveBuild, refusalToReplaceProduction, stampOfMainCommit } from '../../scripts/production-guard';

const upToDate = {
	containsOriginMain: true,
	liveMainCommit: 'abc1234',
	treeHasLiveMainCommit: true,
	isReplacingNewerAllowed: false
};

describe('replacing what production is serving', () => {
	test('a tree holding the main production was built from may deploy', () => {
		expect(refusalToReplaceProduction(upToDate)).toBeNull();
	});

	test('a tree missing that commit is refused', () => {
		const refusal = refusalToReplaceProduction({ ...upToDate, treeHasLiveMainCommit: false });
		expect(refusal).toContain('abc1234');
		expect(refusal).toContain('--replace-newer');
	});

	test('a tree behind origin/main is refused before anything else', () => {
		expect(refusalToReplaceProduction({ ...upToDate, containsOriginMain: false })).toContain('origin/main');
	});

	test('putting an older build back is allowed when it is asked for', () => {
		expect(
			refusalToReplaceProduction({ ...upToDate, treeHasLiveMainCommit: false, isReplacingNewerAllowed: true })
		).toBeNull();
	});

	test('being behind main still refuses even with --replace-newer', () => {
		expect(
			refusalToReplaceProduction({ ...upToDate, containsOriginMain: false, isReplacingNewerAllowed: true })
		).toContain('origin/main');
	});

	test('a build that carries no stamp does not block the next one', () => {
		expect(refusalToReplaceProduction({ ...upToDate, liveMainCommit: null, treeHasLiveMainCommit: true })).toBeNull();
	});
});

describe('the stamp a deploy leaves behind', () => {
	test('round trips', () => {
		expect(mainCommitOfLiveBuild(stampOfMainCommit('4b53d3dcbdc2c4747bc6d13883f35d02851d6a9b'))).toBe(
			'4b53d3dcbdc2c4747bc6d13883f35d02851d6a9b'
		);
	});

	test('reads nothing out of a build that predates it', () => {
		expect(mainCommitOfLiveBuild(null)).toBeNull();
		expect(mainCommitOfLiveBuild('Deploy from CI')).toBeNull();
	});

	test('records main rather than the branch, so a squash merge does not read as a lost build', () => {
		expect(stampOfMainCommit('4b53d3dc')).toBe('origin/main 4b53d3dc');
	});
});
