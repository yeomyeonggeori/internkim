export type ProductionState = {
	containsOriginMain: boolean;
	liveCommit: string | null;
	treeHasLiveCommit: boolean;
	isReplacingNewerAllowed: boolean;
};

export function refusalToReplaceProduction(state: ProductionState): string | null {
	if (!state.containsOriginMain) {
		return [
			'this tree does not contain origin/main, so deploying it would drop merged work.',
			'  git fetch origin && git rebase origin/main'
		].join('\n');
	}
	if (state.isReplacingNewerAllowed || state.liveCommit === null || state.treeHasLiveCommit) return null;
	return [
		`production is serving ${state.liveCommit}, which this tree does not have.`,
		'  Another deploy landed after you built. Rebuild from a tree that has it:',
		'  git fetch origin && git rebase origin/main && bun run build',
		'  Deliberately putting an older build back is --replace-newer.'
	].join('\n');
}
