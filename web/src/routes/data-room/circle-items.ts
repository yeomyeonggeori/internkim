import type { z } from 'zod';
import { categoryName, circleName, type DataRoomCategory } from '$lib/data-room/model';
import type { circleListResultSchema } from '$lib/data-room/schemas';

type Circle = z.infer<typeof circleListResultSchema>['circles'][number];

export type CircleItem = { value: string; label: string };

export function shareableCircleItems(circles: Circle[], shareableCircleIDs: string[], locale: string): CircleItem[] {
	return circles
		.filter((circle) => shareableCircleIDs.includes(circle.id))
		.map((circle) => ({ value: circle.id, label: circleName(circle, locale) }));
}

export function circleLabel(circles: Circle[], circleID: string, locale: string): string {
	const circle = circles.find((candidate) => candidate.id === circleID);
	return circleName(circle ?? { id: circleID, name: null }, locale);
}

export function readableCategoryNames(circle: Circle | undefined, categories: DataRoomCategory[], locale: string): string {
	if (!circle) return '';
	return circle.readableCategories
		.map((code) => {
			const category = categories.find((candidate) => candidate.code === code);
			return category ? categoryName(category, locale) : code;
		})
		.join(', ');
}
