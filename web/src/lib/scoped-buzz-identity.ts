import { sameCompanySessionScope, type StoredCompanyScope } from '$lib/company-session-scope';
export type BuzzIdentityScope = StoredCompanyScope;
export { sameCompanySessionScope as sameBuzzIdentityScope };

export function createScopedBuzzIdentity<Scope extends BuzzIdentityScope>(ports: {
	readScope: () => Promise<Scope | null>;
	claim: (scope: Scope) => Promise<string | null>;
	restore: (scope: Scope) => string | null;
	publish: (scope: Scope, secret: string) => void;
	hide: () => void;
}) {
	let generation = 0;
	let pending: Promise<string | null> | undefined;

	async function load(started: number): Promise<string | null> {
		try {
			const scope = await ports.readScope();
			if (started !== generation || !scope) return null;
			const secret = ports.restore(scope) ?? await ports.claim(scope);
			if (started !== generation || !secret) return null;
			const current = await ports.readScope();
			if (started !== generation || !current || !sameCompanySessionScope(scope, current)) return null;
			ports.publish(scope, secret);
			return secret;
		} catch {
			return null;
		}
	}

	return {
		ensure(): Promise<string | null> {
			if (pending) return pending;
			ports.hide();
			const attempt = load(generation);
			pending = attempt;
			void attempt.finally(() => { if (pending === attempt) pending = undefined; });
			return attempt;
		},
		invalidate(): void {
			generation += 1;
			pending = undefined;
			ports.hide();
		}
	};
}
