export function calendarDateKey(date: Date): string {
	return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`;
}

export function calendarDateFromKey(value: string): Date {
	const [yearText, monthText, dayText] = value.split('-');
	return new Date(Number(yearText), Number(monthText) - 1, Number(dayText), 12, 0, 0, 0);
}
