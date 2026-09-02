const passwordStepLifetimeMilliseconds = 10 * 60 * 1000;

export function hasThePasswordStepExpired(openedAt: number, now: number): boolean {
	return now - openedAt > passwordStepLifetimeMilliseconds;
}
