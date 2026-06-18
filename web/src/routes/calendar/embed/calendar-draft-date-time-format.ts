export type DraftDateTimePickerKind = 'start' | 'end';

export type DraftDateTimePickerValue = {
	dateKey: string;
	time: string;
};

export type DraftDateCell = {
	dateKey: string;
	day: number;
	isCurrentMonth: boolean;
};

export type DraftDateWeek = DraftDateCell[];

export type DraftDateTimePickerLocaleText = {
	previousMonth: string;
	nextMonth: string;
	selectDate: string;
	timeRange: string;
	hour: string;
	minute: string;
	hourSuffix: string;
	minuteSuffix: string;
	save: string;
	editStartDate: string;
	editEndDate: string;
	editStartDateTime: string;
	editEndDateTime: string;
};

export type DraftDateTimeLabelText = {
	startDate: string;
	endDate: string;
	dateTimePicker: DraftDateTimePickerLocaleText;
};

const hours = Array.from({ length: 24 }, (_, hour) => String(hour).padStart(2, '0'));
const minutes = Array.from({ length: 12 }, (_, index) => String(index * 5).padStart(2, '0'));

export function draftDateTimeSummary(dateKey: string, time: string, allDay: boolean): string {
	const dateText = dateKey.replaceAll('-', '.');
	return allDay ? dateText : `${dateText} ${time}`;
}

export function draftDateTimePickerLabel(
	kind: DraftDateTimePickerKind,
	allDay: boolean,
	text: DraftDateTimeLabelText
): string {
	if (kind === 'start') return allDay ? text.dateTimePicker.editStartDate : text.dateTimePicker.editStartDateTime;
	return allDay ? text.dateTimePicker.editEndDate : text.dateTimePicker.editEndDateTime;
}

export function draftDateTimeKindLabel(kind: DraftDateTimePickerKind, text: DraftDateTimeLabelText): string {
	return kind === 'start' ? text.startDate : text.endDate;
}

export function draftDateTimeDateLabel(dateKey: string, localeCode: string): string {
	const [year = '1970', month = '1', day = '1'] = dateKey.split('-');
	return new Date(Number(year), Number(month) - 1, Number(day)).toLocaleDateString(localeCode, {
		year: 'numeric',
		month: 'long',
		day: 'numeric'
	});
}

export function draftDateTimeHours(): string[] {
	return hours;
}

export function draftDateTimeMinutes(): string[] {
	return minutes;
}

export function draftDateTimeMonthTitle(date: Date, localeCode: string): string {
	return date.toLocaleDateString(localeCode, {
		year: 'numeric',
		month: 'long'
	});
}

export function draftDateTimeMonthWeeks(monthDate: Date): DraftDateWeek[] {
	const firstDay = new Date(monthDate.getFullYear(), monthDate.getMonth(), 1);
	const gridStart = new Date(firstDay);
	gridStart.setDate(firstDay.getDate() - firstDay.getDay());
	return Array.from({ length: 6 }, (_, weekIndex) =>
		Array.from({ length: 7 }, (_, dayIndex) => {
			const date = new Date(gridStart);
			date.setDate(gridStart.getDate() + weekIndex * 7 + dayIndex);
			return {
				dateKey: dateKeyFromDate(date),
				day: date.getDate(),
				isCurrentMonth: date.getMonth() === monthDate.getMonth()
			};
		})
	);
}

export function monthDateFromDateKey(dateKey: string): Date {
	const [year = '1970', month = '1'] = dateKey.split('-');
	return new Date(Number(year), Number(month) - 1, 1);
}

export function shiftedMonth(date: Date, monthDelta: number): Date {
	return new Date(date.getFullYear(), date.getMonth() + monthDelta, 1);
}

export function normalizedPickerTime(time: string): { hour: string; minute: string } {
	const [hour = '00', minute = '00'] = time.split(':');
	const roundedMinute = Math.min(55, Math.max(0, Math.round(Number(minute) / 5) * 5));
	return {
		hour: String(Math.min(23, Math.max(0, Number(hour)))).padStart(2, '0'),
		minute: String(roundedMinute).padStart(2, '0')
	};
}

export function pickerTimeValue(hour: string, minute: string): string {
	return `${hour}:${minute}`;
}

function dateKeyFromDate(date: Date): string {
	return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`;
}
