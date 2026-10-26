import { z } from 'zod';
import type { Task } from '../../routes/task/task-types';

const count = z.number().int().nonnegative();
const previewPersonSchema = z.object({ person: z.string(), occurredAt: z.string().nullable() });
const previewPeopleSchema = z.array(previewPersonSchema);

const activityTeamSchema = z.object({
	teamKey: z.string(),
	name: z.string(),
	memberCount: count,
	working: count,
	done: count,
	away: count,
	needsCheckout: count,
	notStarted: count,
	unknownLocationCount: count,
	recordedLocations: z.array(z.object({ name: z.string(), count })),
	preview: z.object({
		working: previewPeopleSchema,
		done: previewPeopleSchema,
		away: previewPeopleSchema,
		notStarted: previewPeopleSchema
	})
});

const activityTaskSchema = z.object({
	id: z.string(),
	title: z.string(),
	status: z.string(),
	business: z.string().optional(),
	type: z.string().optional(),
	size: z.string().optional(),
	startDate: z.string().optional(),
	endDate: z.string().optional(),
	createdAt: z.string().optional(),
	people: z.array(z.string())
});

const namedColorSchema = z.object({ name: z.string(), color: z.string().optional() });

const activityBaseSchema = z.object({
	today: z.string(),
	attendance: z.object({
		date: z.string(),
		summary: z.object({ memberCount: count, working: count, done: count, away: count, notStarted: count, needsCheckout: count }),
		teams: z.array(activityTeamSchema)
	}),
	board: z.object({
		vocabulary: z.object({
			businesses: z.array(namedColorSchema).optional(),
			types: z.array(namedColorSchema).optional()
		}),
		counts: z.record(z.string(), count),
		tasks: z.array(activityTaskSchema)
	}),
	contributions: z.array(z.object({ date: z.string(), count }))
});

export const storedActivitySchema = activityBaseSchema.extend({
	people: z.record(z.string(), z.object({ name: z.string(), picture: z.string().optional() }))
});

export const companyActivitySchema = activityBaseSchema.extend({
	people: z.record(z.string(), z.object({ name: z.string(), image: z.string().optional() }))
});

export type StoredActivity = z.infer<typeof storedActivitySchema>;
export type CompanyActivity = z.infer<typeof companyActivitySchema>;
export type ActivityTeam = CompanyActivity['attendance']['teams'][number];
export type ActivityPerson = CompanyActivity['people'][string];
export type ContributionDay = { date: string; count: number; level: number };
export type ContributionWeek = { days: (ContributionDay | null)[]; month: string };

const contributionLevels = 4;
const daysPerWeek = 7;

export function activityTasks(activity: CompanyActivity): Task[] {
	return activity.board.tasks.map((task) => {
		const names = task.people.map((person) => activity.people[person]?.name ?? '');
		return {
			id: task.id,
			ownerID: task.people[0] ?? '',
			ownerName: names[0] ?? '',
			participantIDs: task.people,
			participantNames: names,
			business: task.business ?? null,
			type: task.type ?? null,
			content: task.title,
			size: task.size ?? '',
			status: task.status,
			startDate: task.startDate,
			endDate: task.endDate,
			createdAt: task.createdAt,
			weekCode: ''
		};
	});
}

export function contributionWeeks(activity: CompanyActivity, weekCount: number): ContributionWeek[] {
	const counts = new Map(activity.contributions.map((day) => [day.date, day.count]));
	const thresholds = levelThresholds([...counts.values()]);
	const today = dateOf(activity.today);
	const lastSunday = addDays(today, daysPerWeek - 1 - mondayIndex(today));
	const firstMonday = addDays(lastSunday, -(weekCount * daysPerWeek - 1));
	return Array.from({ length: weekCount }, (_, week) => {
		const monday = addDays(firstMonday, week * daysPerWeek);
		const days = Array.from({ length: daysPerWeek }, (_, offset) => {
			const day = addDays(monday, offset);
			if (day > today) return null;
			const date = isoDate(day);
			const dayCount = counts.get(date) ?? 0;
			return { date, count: dayCount, level: contributionLevel(dayCount, thresholds) };
		});
		return { days, month: monday.getUTCDate() <= daysPerWeek ? String(monday.getUTCMonth() + 1) : '' };
	});
}

export function contributionTotal(activity: CompanyActivity): number {
	return activity.contributions.reduce((sum, day) => sum + day.count, 0);
}

function levelThresholds(counts: number[]): number[] {
	const busyDays = counts.filter((dayCount) => dayCount > 0).toSorted((left, right) => left - right);
	return Array.from({ length: contributionLevels - 1 }, (_, index) =>
		busyDays[Math.floor(((index + 1) / contributionLevels) * busyDays.length)] ?? Number.POSITIVE_INFINITY
	);
}

function contributionLevel(dayCount: number, thresholds: number[]): number {
	if (dayCount === 0) return 0;
	return 1 + thresholds.filter((threshold) => dayCount >= threshold).length;
}

export type RedactedBar = { redactedLength: number; widthScale: number; outline: [number, number][]; tiltDegrees: number };
export type RedactedSegment = { text: string } | RedactedBar;

export function redactedSegments(seed: string, masked: string): RedactedSegment[] {
	const next = seededRandom(seed);
	return masked
		.split(/(•+)/)
		.filter(Boolean)
		.map((text) => (text.startsWith('•') ? redactedBar(text.length, next) : { text }));
}

function redactedBar(length: number, next: () => number): RedactedBar {
	const wobble = 7 / Math.sqrt(length);
	const steps = Math.max(3, Math.round(Math.sqrt(length) * 2));
	const jitter = (from: number, spread: number): number => Math.round((from + next() * spread) * 10) / 10;
	const top = Array.from({ length: steps + 1 }, (_, index): [number, number] => [(index / steps) * 100, jitter(4, wobble)]);
	const bottom = Array.from({ length: steps + 1 }, (_, index): [number, number] => [100 - (index / steps) * 100, jitter(96 - wobble, wobble)]);
	const startEnd: [number, number] = [jitter(0, wobble / 3), jitter(40, wobble * 2)];
	const finishEnd: [number, number] = [100 - jitter(0, wobble / 3), jitter(45, wobble * 2)];
	return {
		redactedLength: length,
		widthScale: 0.92 + next() * 0.16,
		outline: [startEnd, ...top, finishEnd, ...bottom],
		tiltDegrees: Math.round((next() - 0.5) * (2.4 / Math.sqrt(length)) * 10) / 10
	};
}

function seededRandom(seed: string): () => number {
	let state = [...seed].reduce((hash, character) => Math.imul(hash ^ character.charCodeAt(0), 16777619), 2166136261);
	return () => {
		state = (state + 0x6d2b79f5) | 0;
		let mixed = Math.imul(state ^ (state >>> 15), 1 | state);
		mixed ^= mixed + Math.imul(mixed ^ (mixed >>> 7), 61 | mixed);
		return ((mixed ^ (mixed >>> 14)) >>> 0) / 4294967296;
	};
}

function dateOf(isoDay: string): Date {
	return new Date(`${isoDay}T00:00:00Z`);
}

function isoDate(day: Date): string {
	return day.toISOString().slice(0, 10);
}

function addDays(day: Date, days: number): Date {
	return new Date(day.getTime() + days * 86_400_000);
}

function mondayIndex(day: Date): number {
	return (day.getUTCDay() + 6) % daysPerWeek;
}
