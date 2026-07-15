const minuteMilliseconds = 60_000;

export function startAttendanceMinuteClock(
	onTimeChange: (currentTime: Date) => void,
	getCurrentTime: () => Date = () => new Date()
): () => void {
	let timeoutID: ReturnType<typeof setTimeout>;

	function scheduleNextMinute(currentTime: Date): void {
		const elapsedMilliseconds = currentTime.getSeconds() * 1_000 + currentTime.getMilliseconds();
		timeoutID = setTimeout(() => {
			const nextTime = getCurrentTime();
			onTimeChange(nextTime);
			scheduleNextMinute(nextTime);
		}, minuteMilliseconds - elapsedMilliseconds);
	}

	const currentTime = getCurrentTime();
	onTimeChange(currentTime);
	scheduleNextMinute(currentTime);
	return () => clearTimeout(timeoutID);
}
