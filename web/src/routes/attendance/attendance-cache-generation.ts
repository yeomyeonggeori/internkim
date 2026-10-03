let generation = 0;

export function attendanceCacheGeneration(): number {
	return generation;
}

export function invalidateAttendanceCacheGeneration(): void {
	generation += 1;
}
