export function crmLabel(labels: object, value: string): string {
	const dictionary = labels as Record<string, unknown>;
	return typeof dictionary[value] === 'string' ? dictionary[value] : value;
}
