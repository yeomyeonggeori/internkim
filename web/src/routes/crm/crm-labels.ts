export function crmDefinitionLabel(
	definitions: readonly { id: string; name: string }[],
	fallback: object,
	value: string
): string {
	const named = definitions.find((definition) => definition.id === value);
	if (named && named.name !== named.id) return named.name;
	return crmLabel(fallback, value);
}

export function crmLabel(labels: object, value: string): string {
	const dictionary = labels as Record<string, unknown>;
	return typeof dictionary[value] === 'string' ? dictionary[value] : value;
}
