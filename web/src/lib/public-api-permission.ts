export const publicAPIPermissions = ['read', 'write', 'delete'] as const;

export type PublicAPIPermission = (typeof publicAPIPermissions)[number];

export const fullPublicAPIPermission: PublicAPIPermission = 'delete';

export function publicAPIPermissionOf(presented: unknown): PublicAPIPermission | null {
	return publicAPIPermissions.find((permission) => permission === presented) ?? null;
}
