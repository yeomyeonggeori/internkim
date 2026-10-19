import { z } from 'zod';

export const momentSchema = z
	.string()
	.regex(/^\d{4}(-(H[12]|Q[1-4]|(0[1-9]|1[0-2])(-\d{2})?))?$/)
	.describe('When something happened, as precisely as known: YYYY, YYYY-H1, YYYY-Q3, YYYY-MM or YYYY-MM-DD.');

export type Moment = z.infer<typeof momentSchema>;

export type MomentPrecision = 'year' | 'half' | 'quarter' | 'month' | 'day';

export type MomentSpan = {
	precision: MomentPrecision;
	year: number;
	part: number;
	start: string;
	end: string;
};

const momentGrammar: [MomentPrecision, RegExp][] = [
	['year', /^(\d{4})$/],
	['half', /^(\d{4})-H([12])$/],
	['quarter', /^(\d{4})-Q([1-4])$/],
	['month', /^(\d{4})-(0[1-9]|1[0-2])$/],
	['day', /^(\d{4})-(\d{2})-(\d{2})$/]
];

const monthsIn: Record<Exclude<MomentPrecision, 'day'>, number> = { year: 12, half: 6, quarter: 3, month: 1 };

export function momentSpanOf(text: string | null | undefined): MomentSpan | undefined {
	const written = text?.trim() ?? '';
	for (const [precision, grammar] of momentGrammar) {
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

function daySpan(written: string, year: number, month: number): MomentSpan | undefined {
	if (isoDay(year, month, Number(written.slice(8))) !== written) return undefined;
	return { precision: 'day', year, part: month, start: written, end: written };
}

function isoDay(year: number, month: number, day: number): string {
	return new Date(Date.UTC(year, month - 1, day)).toISOString().slice(0, 10);
}

export function compareMomentsNewestFirst(left: Moment | null | undefined, right: Moment | null | undefined): number {
	const leftSpan = momentSpanOf(left);
	const rightSpan = momentSpanOf(right);
	if (!leftSpan || !rightSpan) return Number(!leftSpan) - Number(!rightSpan);
	return rightSpan.start.localeCompare(leftSpan.start) || rightSpan.end.localeCompare(leftSpan.end);
}

export function momentLabel(text: Moment | null | undefined, locale: string): string {
	const span = momentSpanOf(text);
	if (!span) return text ?? '';
	const isKorean = locale.startsWith('ko');
	if (span.precision === 'year') return String(span.year);
	if (span.precision === 'half') {
		if (isKorean) return `${span.year} ${span.part === 1 ? '상반기' : '하반기'}`;
		return `H${span.part} ${span.year}`;
	}
	if (span.precision === 'quarter') return isKorean ? `${span.year} ${span.part}분기` : `Q${span.part} ${span.year}`;
	return span.start.slice(0, span.precision === 'month' ? 7 : 10).replaceAll('-', '.');
}
