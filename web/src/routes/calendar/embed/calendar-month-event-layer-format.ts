import type {
	MonthEventPlacement,
	MonthMorePlacement
} from './calendar-month-event-geometry';

type MonthMoreHiddenSegment = MonthMorePlacement['hiddenSegments'][number];

export function monthEventPlacementStyle(placement: MonthEventPlacement): string {
	return `left: ${placement.left}px; top: ${placement.top}px; width: ${placement.width}px; height: ${placement.height}px;`;
}

export function monthMorePlacementStyle(placement: MonthMorePlacement): string {
	return `left: ${placement.left}px; top: ${placement.top}px; width: ${placement.width}px; height: ${placement.height}px;`;
}

export function monthMorePopoverStyle(placement: MonthMorePlacement): string {
	const top = placement.top + placement.height + 4;
	return `left: ${placement.left}px; top: ${top}px; min-width: ${Math.min(240, Math.max(160, placement.width))}px;`;
}

export function monthMoreEventDateText(
	segment: MonthEventPlacement | MonthMoreHiddenSegment,
	localeCode: string
): string {
	const dateText =
		segment.startDateKey === segment.endDateKey
			? formattedMonthMoreDate(segment.startDateKey, localeCode)
			: `${formattedMonthMoreDate(segment.startDateKey, localeCode)}-${formattedMonthMoreDate(segment.endDateKey, localeCode)}`;
	if (!segment.startTimeText) return dateText;
	return `${dateText} ${segment.startTimeText}`;
}

export function monthMoreButtonText(template: string, count: number): string {
	return template.replace('{count}', String(count));
}

export function monthMoreAriaLabel(template: string, placement: MonthMorePlacement, localeCode: string): string {
	return template
		.replace('{count}', String(placement.count))
		.replace('{date}', formattedMonthMoreDate(placement.dateKey, localeCode));
}

export function formattedMonthMoreDate(dateKey: string, localeCode: string): string {
	const date = monthMoreDateFromDateKey(dateKey);
	if (!date) return dateKey;
	return new Intl.DateTimeFormat(localeCode, { month: 'numeric', day: 'numeric' }).format(date);
}

function monthMoreDateFromDateKey(dateKey: string): Date | null {
	const [yearText, monthText, dayText] = dateKey.split('-');
	const year = Number(yearText);
	const month = Number(monthText);
	const day = Number(dayText);
	if (!Number.isInteger(year) || !Number.isInteger(month) || !Number.isInteger(day)) return null;
	return new Date(year, month - 1, day);
}
