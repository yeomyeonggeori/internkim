import { reachesPermission, type PublicAPIPermission } from '$lib/public-api-permission';

export const longestTokenName = 64;
const unnamedTokenPrefix = 'pat-';

export function nextUnusedName(held: { name: string }[]): string {
	const taken = new Set(held.map((token) => token.name));
	for (let ordinal = 1; ; ordinal += 1) {
		const name = `${unnamedTokenPrefix}${ordinal}`;
		if (!taken.has(name)) return name;
	}
}

export type TokenRefusal = { status: number; message: string };

export function issueRefusal(
	caller: { permission: PublicAPIPermission; tokenName: string },
	name: string,
	permission: PublicAPIPermission
): TokenRefusal | null {
	if (name.length > longestTokenName) return { status: 400, message: 'that name is too long for a token' };
	if (name === caller.tokenName && name !== '') {
		return { status: 409, message: 'that name belongs to the token making this call' };
	}
	if (!reachesPermission(caller.permission, permission)) {
		return { status: 403, message: `this token may not make one that ${permission}s` };
	}
	return null;
}

export function revocationRefusal(caller: { tokenName: string }, name: string): TokenRefusal | null {
	if (!name) return { status: 400, message: 'a revocation names the token' };
	if (name === caller.tokenName) {
		return { status: 409, message: 'a token cannot revoke itself; revoke it with the one that replaced it' };
	}
	return null;
}
