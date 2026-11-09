export function creditCount(credits: number, locale: string): string {
	return credits.toLocaleString(locale);
}

export function billingDay(isoDate: string | null, locale: string): string {
	if (!isoDate) return '';
	return new Date(isoDate).toLocaleDateString(locale, { month: 'long', day: 'numeric' });
}

export function filled(template: string, values: Record<string, string>): string {
	return Object.entries(values).reduce((text, [name, value]) => text.replaceAll(`{${name}}`, value), template);
}
