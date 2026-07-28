export type FlowWeekDateRange = {
	startISO: string;
	endISO: string;
};

const millisecondsPerDay = 24 * 60 * 60 * 1000;

export function formatFlowWeekDateRange(week: FlowWeekDateRange | null | undefined): string {
	if (!week?.startISO || !week.endISO) return '...';
	return `${formatMonthDay(week.startISO)} - ${formatMonthDay(week.endISO)}`;
}

export type FlowWeekOption = {
	value: string;
	label: string;
	isCurrent: boolean;
};

export function flowWeekOptions(
	currentWeekStartISO: string,
	weeksBefore: number,
	weeksAfter: number
): FlowWeekOption[] {
	const currentMonday = dateFromDateISO(currentWeekStartISO);
	if (!currentMonday) return [];
	const options: FlowWeekOption[] = [];
	for (let offset = weeksBefore; offset >= -weeksAfter; offset -= 1) {
		const monday = addUTCDate(currentMonday, -offset * 7);
		const sunday = addUTCDate(monday, 6);
		options.push({
			value: weekCodeForMonday(monday),
			label: formatFlowWeekDateRange({ startISO: dateISOFromDate(monday), endISO: dateISOFromDate(sunday) }),
			isCurrent: offset === 0
		});
	}
	return options;
}

export function flowWeekCodeForDateISO(dateISO: string): string {
	const date = dateFromDateISO(dateISO);
	if (!date) return '';
	const monday = addUTCDate(date, -((date.getUTCDay() + 6) % 7));
	return weekCodeForMonday(monday);
}

function formatMonthDay(dateISO: string): string {
	const month = Number(dateISO.slice(5, 7));
	const day = Number(dateISO.slice(8, 10));
	if (!month || !day) return dateISO;
	return `${month}/${day}`;
}

function dateISOFromDate(date: Date): string {
	return date.toISOString().slice(0, 10);
}

function dateFromDateISO(dateISO: string): Date | null {
	const matched = /^(\d{4})-(\d{2})-(\d{2})$/.exec(dateISO);
	if (!matched) return null;
	const year = Number(matched[1]);
	const month = Number(matched[2]);
	const day = Number(matched[3]);
	const date = new Date(Date.UTC(year, month - 1, day));
	if (date.getUTCFullYear() !== year) return null;
	if (date.getUTCMonth() !== month - 1) return null;
	if (date.getUTCDate() !== day) return null;
	return date;
}

function weekCodeForMonday(monday: Date): string {
	const thursday = addUTCDate(monday, 3);
	const year = thursday.getUTCFullYear();
	const januaryFourth = new Date(Date.UTC(year, 0, 4));
	const isoWeekOneMonday = addUTCDate(januaryFourth, -((januaryFourth.getUTCDay() + 6) % 7));
	const week = Math.floor((monday.getTime() - isoWeekOneMonday.getTime()) / (7 * millisecondsPerDay)) + 1;
	return `${String(year).slice(2)}W${String(week).padStart(2, '0')}`;
}

function addUTCDate(date: Date, days: number): Date {
	return new Date(date.getTime() + days * millisecondsPerDay);
}
