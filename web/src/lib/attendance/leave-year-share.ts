const dayInMilliseconds = 86_400_000;

function dayCountBetween(firstDate: string, lastDate: string): number {
	const first = Date.parse(`${firstDate}T00:00:00Z`);
	const last = Date.parse(`${lastDate}T00:00:00Z`);
	if (Number.isNaN(first) || Number.isNaN(last) || last < first) return 0;
	return Math.round((last - first) / dayInMilliseconds) + 1;
}

/**
 * The share of a leave's deducted days that falls inside one calendar year.
 *
 * A leave spanning new year belongs to both years, in proportion to how many of
 * its days land in each. A leave inside one year keeps all of its days there.
 * The central plane deducts every day of the range, weekends included, so
 * counting days is the whole calculation.
 */
export function leaveDaysInYear(
	totalDays: number,
	localStartDate: string,
	localEndDate: string,
	targetYear: number
): number {
	const span = dayCountBetween(localStartDate, localEndDate);
	if (span === 0) return 0;

	const startYear = Number(localStartDate.slice(0, 4));
	const endYear = Number(localEndDate.slice(0, 4));
	if (startYear === endYear) return startYear === targetYear ? totalDays : 0;
	if (targetYear < startYear || targetYear > endYear) return 0;

	const firstOfTarget = `${targetYear}-01-01`;
	const lastOfTarget = `${targetYear}-12-31`;
	const inYear = dayCountBetween(
		localStartDate > firstOfTarget ? localStartDate : firstOfTarget,
		localEndDate < lastOfTarget ? localEndDate : lastOfTarget
	);
	return (totalDays * inYear) / span;
}
