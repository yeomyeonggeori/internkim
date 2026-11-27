import type { z } from 'zod';
import { companyCreditsResultSchema } from './credits-contract';

const storageKeyPrefix = 'company.my-credits.';

const myCreditUsageSchema = companyCreditsResultSchema.pick({
	paidBy: true,
	periodEndsAt: true,
	usageLimit: true,
	myCreditsUsed: true
});

export type MyCreditUsage = z.infer<typeof myCreditUsageSchema>;

function storageKey(scope: string): string {
	return `${storageKeyPrefix}${encodeURIComponent(scope)}`;
}

export function readStoredMyCredits(scope: string): MyCreditUsage | null {
	if (!scope || typeof window === 'undefined') return null;
	try {
		const raw = window.localStorage.getItem(storageKey(scope));
		if (!raw) return null;
		const stored: unknown = JSON.parse(raw);
		const usage = myCreditUsageSchema.safeParse(stored);
		return usage.success ? usage.data : null;
	} catch {
		return null;
	}
}

export function storeMyCredits(scope: string, usage: MyCreditUsage): void {
	if (!scope || typeof window === 'undefined') return;
	const { paidBy, periodEndsAt, usageLimit, myCreditsUsed } = usage;
	try {
		window.localStorage.setItem(storageKey(scope), JSON.stringify({ paidBy, periodEndsAt, usageLimit, myCreditsUsed }));
	} catch {
		return;
	}
}

export function forgetStoredMyCredits(): void {
	if (typeof window === 'undefined') return;
	try {
		const storage = window.localStorage;
		const keys = Array.from({ length: storage.length }, (_, index) => storage.key(index)).filter(
			(key): key is string => Boolean(key?.startsWith(storageKeyPrefix))
		);
		for (const key of keys) storage.removeItem(key);
	} catch {
		return;
	}
}
