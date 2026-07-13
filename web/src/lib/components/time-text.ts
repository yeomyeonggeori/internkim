const DISPLAY_TIME_PATTERN = /^(?:[01]\d|2[0-4]):[0-5]\d(?::[0-5]\d(?:\.\d+)?)?$/;

export function formatDisplayTime(value: string): string {
	if (!DISPLAY_TIME_PATTERN.test(value)) return value;
	return value.slice(0, 5);
}
