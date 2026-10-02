import { onCompanyBroadcast } from '$lib/company-channel';
import { clearStoredTaskSnapshot } from '../../routes/task/task-snapshot-storage';

const refreshDelayMilliseconds = 400;

export function subscribeTaskWrites(onWrite: () => void): () => void {
	let refreshTimer: ReturnType<typeof setTimeout> | undefined;
	const stopListening = onCompanyBroadcast('task_written', () => {
		clearStoredTaskSnapshot();
		clearTimeout(refreshTimer);
		refreshTimer = setTimeout(onWrite, refreshDelayMilliseconds);
	});
	return () => {
		stopListening();
		clearTimeout(refreshTimer);
	};
}
