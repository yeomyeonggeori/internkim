export function personAvatarSeed(email: string, seed: string, name: string): string {
	return email.trim().toLowerCase() || seed.trim() || name.trim() || '?';
}
