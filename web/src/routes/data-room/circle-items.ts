import type { z } from 'zod';
import { circleName } from '$lib/data-room/model';
import type { circleListResultSchema } from '$lib/data-room/schemas';

type Circle = z.infer<typeof circleListResultSchema>['circles'][number];

export type CircleItem = { value: string; label: string };

export function shareableCircleItems(circles: Circle[], shareableCircleIDs: string[], locale: string): CircleItem[] {
	return circles
		.filter((circle) => shareableCircleIDs.includes(circle.id))
		.map((circle) => ({ value: circle.id, label: circleName(circle, locale) }));
}
