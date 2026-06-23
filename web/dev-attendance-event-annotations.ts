import type { AttendanceKind } from './src/routes/attendance/attendance-context.svelte';

export type Scenario = 'normal' | 'ambiguous' | 'overridden' | 'manual';

export type Annotation = {
	sourceMessage?: string;
	confidence?: number;
	parsedAs?: { kind?: AttendanceKind; locationID?: string };
	overriddenBy?: string;
	overriddenAt?: string;
	manualEntry?: boolean;
};

const CLOCK_IN_MESSAGES: Record<string, string[]> = {
	office: ['사무실 도착했어요', '오늘 시작합니다', '사무실 출근'],
	remote: ['오늘 재택할게요', '재택 시작합니다', '집에서 일합니다'],
	bss: ['BSS 도착했어요', 'BSS에서 시작합니다', 'BSS 출근'],
	outside: ['외부 출발합니다', '현장 도착했어요', '외부 일정 시작'],
};

const CLOCK_OUT_MESSAGES = ['퇴근합니다', '오늘은 여기까지', '퇴근할게요'];

const AMBIGUOUS_MESSAGES = [
	'좀 늦어요',
	'오늘 컨디션이 안 좋네요',
	'조금 이따 시작할게요',
	'잠깐 자리 비울게요',
];

const AMBIGUOUS_CLOCK_OUT_MESSAGES = [
	'이제 갈게요',
	'잠시 자리 비울게요',
	'오늘은 일찍 마칠게요',
	'잠깐 외출',
];

export function annotateClockIn(scenario: Scenario, locationID: string, seed: number, date: string): Annotation {
	if (scenario === 'manual') return { manualEntry: true };
	if (scenario === 'ambiguous') {
		return {
			sourceMessage: pickMessage(AMBIGUOUS_MESSAGES, seed),
			confidence: 0.48,
			parsedAs: { kind: 'clock_in', locationID },
		};
	}
	if (scenario === 'overridden') {
		const messagePool = CLOCK_IN_MESSAGES[locationID] ?? CLOCK_IN_MESSAGES.office;
		const initialGuess = locationID === 'office' ? 'remote' : 'office';
		return {
			sourceMessage: pickMessage(messagePool, seed),
			confidence: 0.71,
			parsedAs: { kind: 'clock_in', locationID: initialGuess },
			overriddenBy: 'kim@example.com',
			overriddenAt: `${date}T10:30:00+09:00`,
		};
	}
	const pool = CLOCK_IN_MESSAGES[locationID] ?? CLOCK_IN_MESSAGES.office;
	return {
		sourceMessage: pickMessage(pool, seed),
		confidence: 0.92,
		parsedAs: { kind: 'clock_in', locationID },
	};
}

export function annotateClockOut(scenario: Scenario, locationID: string, seed: number, date: string): Annotation {
	if (scenario === 'manual') return { manualEntry: true };
	if (scenario === 'ambiguous') {
		return {
			sourceMessage: pickMessage(AMBIGUOUS_CLOCK_OUT_MESSAGES, seed),
			confidence: 0.5,
			parsedAs: { kind: 'clock_out', locationID },
		};
	}
	if (scenario === 'overridden') {
		return {
			sourceMessage: pickMessage(CLOCK_OUT_MESSAGES, seed),
			confidence: 0.72,
			parsedAs: { kind: 'clock_out', locationID: locationID === 'office' ? 'remote' : 'office' },
			overriddenBy: 'kim@example.com',
			overriddenAt: `${date}T19:00:00+09:00`,
		};
	}
	return {
		sourceMessage: pickMessage(CLOCK_OUT_MESSAGES, seed),
		confidence: 0.93,
		parsedAs: { kind: 'clock_out', locationID },
	};
}

export function scenarioFor(day: number, personIndex: number): Scenario {
	const seed = (day * 7 + personIndex * 13) % 20;
	if (seed === 0) return 'manual';
	if (seed === 1) return 'overridden';
	if (seed === 2 || seed === 3) return 'ambiguous';
	return 'normal';
}

function pickMessage(pool: string[], seed: number): string {
	return pool[seed % pool.length];
}
