export function reasonOf(failure: unknown): string {
	return failure instanceof Error ? failure.message : String(failure);
}

export function doublingDelayMilliseconds(failures: number, firstMilliseconds: number, longestMilliseconds: number): number {
	return Math.min(firstMilliseconds * 2 ** Math.max(failures - 1, 0), longestMilliseconds);
}
