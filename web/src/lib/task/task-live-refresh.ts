import { onCompanyBroadcast } from '$lib/company-channel';

const refreshDelayMilliseconds = 400;

export function subscribeTaskWrites(onWrite: () => void): () => void {
	let refreshTimer: ReturnType<typeof setTimeout> | undefined;
	const stopListening = onCompanyBroadcast('task_written', () => {
		clearTimeout(refreshTimer);
		refreshTimer = setTimeout(onWrite, refreshDelayMilliseconds);
	});
	return () => {
		stopListening();
		clearTimeout(refreshTimer);
	};
}
