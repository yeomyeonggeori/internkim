export const defaultTokenLifetimeDays = 90;

export const longestTokenLifetimeDays = 365;

const dayMilliseconds = 24 * 60 * 60 * 1000;

export function tokenLifetimeDaysOf(presented: unknown): number | null {
	if (presented === undefined) return defaultTokenLifetimeDays;
	if (typeof presented !== 'number' || !Number.isInteger(presented)) return null;
	if (presented < 1 || presented > longestTokenLifetimeDays) return null;
	return presented;
}

export function expiryAfter(lifetimeDays: number, now: Date): string {
	return new Date(now.getTime() + lifetimeDays * dayMilliseconds).toISOString();
}

export function expiresWithin(expiresAt: string, days: number, now: Date): boolean {
	return Date.parse(expiresAt) - now.getTime() < days * dayMilliseconds;
}
