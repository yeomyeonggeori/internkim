export type TaskDateParts = {
	year?: string;
	month: string;
	day: string;
};

export function taskDateParts(date: string | undefined, currentYear: number): TaskDateParts | undefined {
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

export function taskDateText(parts: TaskDateParts): string {
	return [parts.year, parts.month, parts.day].filter(Boolean).join('/');
}
