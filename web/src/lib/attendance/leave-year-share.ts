const dayInMilliseconds = 86_400_000;

function dayCountBetween(firstDate: string, lastDate: string): number {
	const first = Date.parse(`${firstDate}T00:00:00Z`);
	const last = Date.parse(`${lastDate}T00:00:00Z`);
	if (Number.isNaN(first) || Number.isNaN(last) || last < first) return 0;
	return Math.round((last - first) / dayInMilliseconds) + 1;
}

function dateWritten(year: number, month: number, day: number): string {
	return `${String(year).padStart(4, '0')}-${String(month).padStart(2, '0')}-${String(day).padStart(2, '0')}`;
}

function dayBefore(date: string): string {
	const before = new Date(Date.parse(`${date}T00:00:00Z`) - dayInMilliseconds);
	return before.toISOString().slice(0, 10);
}

export function leaveYearOpensOn(
	targetYear: number,
	yearStartMonth = 1,
	yearStartDay = 1
): string {
	return dateWritten(targetYear, yearStartMonth, yearStartDay);
}

export function leaveYearClosesOn(
	targetYear: number,
	yearStartMonth = 1,
	yearStartDay = 1
): string {
	return dayBefore(dateWritten(targetYear + 1, yearStartMonth, yearStartDay));
}

export function leaveYearOf(localDate: string, yearStartMonth = 1, yearStartDay = 1): number {
	const year = Number(localDate.slice(0, 4));
	return localDate < leaveYearOpensOn(year, yearStartMonth, yearStartDay) ? year - 1 : year;
}

export function leaveDaysInYear(
	totalDays: number,
	localStartDate: string,
	localEndDate: string,
	targetYear: number,
	yearStartMonth = 1,
	yearStartDay = 1
): number {
	const lastDate = localEndDate < localStartDate ? localStartDate : localEndDate;
	const span = dayCountBetween(localStartDate, lastDate);
	if (span === 0) return 0;

	const opens = leaveYearOpensOn(targetYear, yearStartMonth, yearStartDay);
	const closes = leaveYearClosesOn(targetYear, yearStartMonth, yearStartDay);
	if (lastDate < opens || localStartDate > closes) return 0;

	const inYear = dayCountBetween(
		localStartDate > opens ? localStartDate : opens,
		lastDate < closes ? lastDate : closes
	);
	return (totalDays * inYear) / span;
}
