const playersBySource = new Map<string, HTMLVideoElement[]>();

function seek(video: HTMLVideoElement, seconds: number): void {
	if (video.readyState >= HTMLMediaElement.HAVE_METADATA) {
		video.currentTime = seconds;
		return;
	}
	video.addEventListener('loadedmetadata', () => (video.currentTime = seconds), { once: true });
}

function handOff(from: HTMLVideoElement, to: HTMLVideoElement): void {
	const wasPlaying = !from.paused;
	from.pause();
	seek(to, from.currentTime);
	if (wasPlaying) void to.play();
}

export function videoPlaybackHandoff(video: HTMLVideoElement, source: string): { destroy(): void } {
	const players = playersBySource.get(source) ?? [];
	const previous = players.at(-1);
	if (previous) handOff(previous, video);
	playersBySource.set(source, [...players, video]);

	return {
		destroy() {
			const remaining = (playersBySource.get(source) ?? []).filter((player) => player !== video);
			const next = remaining.at(-1);
			if (next) handOff(video, next);
			if (remaining.length === 0) playersBySource.delete(source);
			else playersBySource.set(source, remaining);
		}
	};
}
