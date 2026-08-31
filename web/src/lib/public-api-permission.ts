export const publicAPIPermissions = ['read', 'write', 'delete'] as const;

export type PublicAPIPermission = (typeof publicAPIPermissions)[number];

export const fullPublicAPIPermission: PublicAPIPermission = 'delete';

export function publicAPIPermissionOf(presented: unknown): PublicAPIPermission | null {
	return publicAPIPermissions.find((permission) => permission === presented) ?? null;
}

export function reachesPermission(held: PublicAPIPermission, asked: PublicAPIPermission): boolean {
	return publicAPIPermissions.indexOf(asked) <= publicAPIPermissions.indexOf(held);
}
