export const playbackRates = [0.5, 1, 1.25, 1.5, 2];

const secondsPerMinute = 60;

export function formatPlaybackTime(totalSeconds: number): string {
	const wholeSeconds = Number.isFinite(totalSeconds) && totalSeconds > 0 ? Math.floor(totalSeconds) : 0;
	const minutes = Math.floor(wholeSeconds / secondsPerMinute);
	const seconds = wholeSeconds % secondsPerMinute;
	return `${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}`;
}
