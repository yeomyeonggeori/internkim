const minuteMilliseconds = 60_000;

export function startAttendanceMinuteClock(
	onTimeChange: (currentTime: Date) => void,
	getCurrentTime: () => Date = () => new Date()
): () => void {
	let timeoutID: ReturnType<typeof setTimeout> | undefined;

	function scheduleNextMinute(currentTime: Date): void {
		if (!Number.isFinite(currentTime.getTime())) return;
		const elapsedMilliseconds = currentTime.getSeconds() * 1_000 + currentTime.getMilliseconds();
		timeoutID = setTimeout(() => {
			const nextTime = getCurrentTime();
			if (!Number.isFinite(nextTime.getTime())) return;
			onTimeChange(nextTime);
			scheduleNextMinute(nextTime);
		}, minuteMilliseconds - elapsedMilliseconds);
	}

	const currentTime = getCurrentTime();
	if (Number.isFinite(currentTime.getTime())) {
		onTimeChange(currentTime);
		scheduleNextMinute(currentTime);
	}
	return () => {
		if (timeoutID !== undefined) clearTimeout(timeoutID);
	};
}
