export type FlowTaskDateParts = {
	year?: string;
	month: string;
	day: string;
};

export function flowTaskDateParts(date: string | undefined, currentYear: number): FlowTaskDateParts | undefined {
	if (!date) return undefined;
	const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(date);
	if (!match) return undefined;
	const [, year = '', month = '', day = ''] = match;
	return {
		year: Number(year) === currentYear ? undefined : year,
		month,
		day
	};
}

export function flowTaskDateText(parts: FlowTaskDateParts): string {
	return [parts.year, parts.month, parts.day].filter(Boolean).join('/');
}
