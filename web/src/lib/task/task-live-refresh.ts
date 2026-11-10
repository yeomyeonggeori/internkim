import { onCompanyBroadcast } from '$lib/company-channel';
import { markTaskSnapshotStale, taskSnapshotGeneration } from '../../routes/task/task-snapshot-storage';

const refreshDelayMilliseconds = 400;

export function subscribeTaskWrites(onWrite: (heardGeneration: number) => void): () => void {
	let refreshTimer: ReturnType<typeof setTimeout> | undefined;
	const stopListening = onCompanyBroadcast('task_written', () => {
		markTaskSnapshotStale();
		const heardGeneration = taskSnapshotGeneration();
		clearTimeout(refreshTimer);
		refreshTimer = setTimeout(() => onWrite(heardGeneration), refreshDelayMilliseconds);
	});
	return () => {
		stopListening();
		clearTimeout(refreshTimer);
	};
}
