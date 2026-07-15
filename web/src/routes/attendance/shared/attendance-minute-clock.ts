const minuteMilliseconds = 60_000;

export function startAttendanceMinuteClock(onTimeChange: (currentTime: Date) => void): () => void {
	let timeoutID: ReturnType<typeof setTimeout>;

	function scheduleNextMinute(currentTime: Date): void {
		const elapsedMilliseconds = currentTime.getSeconds() * 1_000 + currentTime.getMilliseconds();
		timeoutID = setTimeout(() => {
			const nextTime = new Date();
			onTimeChange(nextTime);
			scheduleNextMinute(nextTime);
		}, minuteMilliseconds - elapsedMilliseconds);
	}

	const currentTime = new Date();
	onTimeChange(currentTime);
	scheduleNextMinute(currentTime);
	return () => clearTimeout(timeoutID);
}
