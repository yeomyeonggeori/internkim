export function positiveNumberSetting(name: string, given: string | undefined, fallback: number): number {
	const written = given?.trim();
	if (!written) return fallback;
	const value = Number(written);
	if (!Number.isFinite(value) || value <= 0) {
		throw new Error(`${name} must be a positive number, not ${JSON.stringify(written)}`);
	}
	return value;
}
