export type ProductionState = {
	containsOriginMain: boolean;
	liveMainCommit: string | null;
	treeHasLiveMainCommit: boolean;
	isReplacingNewerAllowed: boolean;
};

const mainStamp = /origin\/main ([0-9a-f]{7,40})/;

export function mainCommitOfLiveBuild(commitMessage: string | null): string | null {
	return commitMessage?.match(mainStamp)?.[1] ?? null;
}

export function stampOfMainCommit(commit: string): string {
	return `origin/main ${commit}`;
}

export function refusalToReplaceProduction(state: ProductionState): string | null {
	if (!state.containsOriginMain) {
		return [
			'this tree does not contain origin/main, so deploying it would drop merged work.',
			'  git fetch origin && git rebase origin/main'
		].join('\n');
	}
	if (state.isReplacingNewerAllowed || state.liveMainCommit === null || state.treeHasLiveMainCommit) return null;
	return [
		`production was built from origin/main ${state.liveMainCommit}, which this tree does not have.`,
		'  Another deploy landed after you built. Rebuild from a tree that has it:',
		'  git fetch origin && git rebase origin/main && bun run build',
		'  Deliberately putting an older build back is --replace-newer.'
	].join('\n');
}
