import { z } from 'zod';

export const periodSchema = z
	.string()
	.regex(/^\d{4}(-(H[12]|Q[1-4]|(0[1-9]|1[0-2])(-\d{2})?))?$/)
	.describe('A year, half, quarter, month or day: YYYY, YYYY-H1, YYYY-Q3, YYYY-MM or YYYY-MM-DD.');

export type Period = z.infer<typeof periodSchema>;

export type PeriodPrecision = 'year' | 'half' | 'quarter' | 'month' | 'day';

export type PeriodSpan = {
	precision: PeriodPrecision;
	year: number;
	part: number;
	start: string;
	end: string;
};

const periodGrammar: [PeriodPrecision, RegExp][] = [
	['year', /^(\d{4})$/],
	['half', /^(\d{4})-H([12])$/],
	['quarter', /^(\d{4})-Q([1-4])$/],
	['month', /^(\d{4})-(0[1-9]|1[0-2])$/],
	['day', /^(\d{4})-(\d{2})-(\d{2})$/]
];

const monthsIn: Record<Exclude<PeriodPrecision, 'day'>, number> = { year: 12, half: 6, quarter: 3, month: 1 };

export function periodSpanOf(text: string | null | undefined): PeriodSpan | undefined {
	const written = text?.trim() ?? '';
	for (const [precision, grammar] of periodGrammar) {
		const match = grammar.exec(written);
		if (!match) continue;
		const year = Number(match[1]);
		const part = Number(match[2] ?? 1);
		if (precision === 'day') return daySpan(written, year, part);
		const firstMonth = (part - 1) * monthsIn[precision] + 1;
		return {
			precision,
			year,
			part,
			start: isoDay(year, firstMonth, 1),
			end: isoDay(year, firstMonth + monthsIn[precision], 0)
		};
	}
	return undefined;
}

function daySpan(written: string, year: number, month: number): PeriodSpan | undefined {
	if (isoDay(year, month, Number(written.slice(8))) !== written) return undefined;
	return { precision: 'day', year, part: month, start: written, end: written };
}

function isoDay(year: number, month: number, day: number): string {
	return new Date(Date.UTC(year, month - 1, day)).toISOString().slice(0, 10);
}

export function comparePeriodsNewestFirst(left: Period | null | undefined, right: Period | null | undefined): number {
	const leftPeriod = periodSpanOf(left);
	const rightPeriod = periodSpanOf(right);
	if (!leftPeriod || !rightPeriod) return Number(!leftPeriod) - Number(!rightPeriod);
	return rightPeriod.start.localeCompare(leftPeriod.start) || rightPeriod.end.localeCompare(leftPeriod.end);
}

export function periodLabel(text: Period | null | undefined, locale: string): string {
	const period = periodSpanOf(text);
	if (!period) return text ?? '';
	const isKorean = locale.startsWith('ko');
	if (period.precision === 'year') return String(period.year);
	if (period.precision === 'half') {
		if (isKorean) return `${period.year} ${period.part === 1 ? '상반기' : '하반기'}`;
		return `H${period.part} ${period.year}`;
	}
	if (period.precision === 'quarter') return isKorean ? `${period.year} ${period.part}분기` : `Q${period.part} ${period.year}`;
	return period.start.slice(0, period.precision === 'month' ? 7 : 10).replaceAll('-', '.');
}
