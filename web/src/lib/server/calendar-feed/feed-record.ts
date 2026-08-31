import { controlPlane, type ControlPlaneCredentials } from '../control-plane';
import type { CalendarFeed, CalendarFeedEvent } from './ics-document';

export async function calendarFeedOf(
	credentials: ControlPlaneCredentials,
	feedKey: string
): Promise<CalendarFeed | null> {
	const { data, error } = await controlPlane(credentials).rpc('calendar_feed', {
		feed_key: feedKey
	});
	if (error) throw new Error(`calendar feed: ${error.message}`);
	return feedOf(data);
}

export function feedOf(document: unknown): CalendarFeed | null {
	if (!document || typeof document !== 'object') return null;
	const held = document as { company?: unknown; timezone?: unknown; events?: unknown };
	return {
		company: typeof held.company === 'string' ? held.company : '',
		timezone: typeof held.timezone === 'string' ? held.timezone : 'UTC',
		events: Array.isArray(held.events) ? held.events.flatMap(eventOf) : []
	};
}

function eventOf(row: unknown): CalendarFeedEvent[] {
	if (!row || typeof row !== 'object') return [];
	const held = row as Record<string, unknown>;
	const id = textOf(held.id);
	const startsAt = textOf(held.startsAt);
	const endsAt = textOf(held.endsAt);
	if (!id || !startsAt || !endsAt) return [];
	return [
		{
			id,
			title: textOf(held.title),
			note: optionalTextOf(held.note),
			location: optionalTextOf(held.location),
			startsAt,
			endsAt,
			isWholeDay: held.isWholeDay === true,
			updatedAt: textOf(held.updatedAt) || startsAt
		}
	];
}

function textOf(value: unknown): string {
	return typeof value === 'string' ? value : '';
}

function optionalTextOf(value: unknown): string | null {
	return typeof value === 'string' && value ? value : null;
}
