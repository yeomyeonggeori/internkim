import { dayIn, dayOfInstant, isADay, momentIn } from './days';
import type { RecordContext } from './company';
import type { TaskRow } from './tasks';

export type ActivityDates = {
	isEvent?: boolean;
	isWholeDay?: boolean;
	startsAt?: string;
	endsAt?: string;
};

type DatesWritten = Pick<ActivityDates, 'isWholeDay' | 'startsAt' | 'endsAt'>;

function named(dates: DatesWritten): DatesWritten {
	return {
		...(dates.isWholeDay !== undefined ? { isWholeDay: dates.isWholeDay } : {}),
		...(dates.startsAt ? { startsAt: dates.startsAt } : {}),
		...(dates.endsAt ? { endsAt: dates.endsAt } : {})
	};
}

function datesOfANewActivity(context: RecordContext, asked: ActivityDates): DatesWritten {
	const startsAt = asked.startsAt?.trim() || dayIn(context.labels.timezone, context.now);
	return { isWholeDay: asked.isWholeDay ?? isADay(startsAt), startsAt, endsAt: asked.endsAt?.trim() || startsAt };
}

function heldDatesIn(context: RecordContext, held: TaskRow, isWholeDay: boolean): DatesWritten {
	if (!held.starts_at || !held.ends_at) return {};
	if (!isWholeDay) return { startsAt: held.starts_at, endsAt: held.ends_at };
	const timezone = context.labels.timezone;
	return {
		startsAt: dayOfInstant(timezone, held.starts_at),
		endsAt: held.task_last_day ?? ''
	};
}

export function activityDatesWritten(
	context: RecordContext,
	asked: ActivityDates,
	held: TaskRow | null
): DatesWritten {
	if (!held) return datesOfANewActivity(context, asked);
	if (asked.isEvent && !held.starts_at && !asked.startsAt) return datesOfANewActivity(context, asked);
	const isWholeDay = asked.isWholeDay ?? held.is_whole_day;
	const turnsOver = isWholeDay !== held.is_whole_day;
	const kept = turnsOver ? heldDatesIn(context, held, isWholeDay) : {};
	return named({
		isWholeDay: asked.isWholeDay,
		startsAt: asked.startsAt ?? kept.startsAt,
		endsAt: asked.endsAt ?? kept.endsAt
	});
}

export function activityDatesAnswered(
	context: RecordContext,
	row: TaskRow
): { startsAt: string; endsAt: string } {
	const timezone = context.labels.timezone;
	if (row.is_whole_day) {
		return {
			startsAt: dayOfInstant(timezone, row.starts_at),
			endsAt: row.task_last_day ?? ''
		};
	}
	return {
		startsAt: row.starts_at ? momentIn(timezone, row.starts_at) : '',
		endsAt: row.ends_at ? momentIn(timezone, row.ends_at) : ''
	};
}
