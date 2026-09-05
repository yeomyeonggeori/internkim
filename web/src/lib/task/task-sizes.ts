import type { TaskSizeDefinition } from '../../routes/task/task-types';
import taskSizeDefinitionsDocument from '../../../../internal/tasksize/definitions.json';

type Locale = 'ko' | 'en';

const sizes = taskSizeDefinitionsDocument.sizes;

export function taskSizes(locale: Locale = 'ko'): TaskSizeDefinition[] {
	return sizes.map((size) => {
		const wording = locale === 'en' ? size.en : size.ko;
		return {
			name: size.name,
			distanceKm: size.distanceKm,
			maxHours: size.maxHours,
			score: size.score,
			label: locale === 'en' ? `${size.distanceKm}km · up to ${size.maxHours}h` : `${size.distanceKm}km · 최대 ${size.maxHours}h`,
			...wording
		};
	});
}

export const taskSizeNames = sizes.map((size) => size.name);

export function sizeOfHours(hours: number): string {
	const fitting = sizes.find((size) => hours <= size.maxHours);
	return (fitting ?? sizes[sizes.length - 1]).name;
}

const sizeOfWholeDayNames = sizes.slice(2, 5).map((size) => size.name);

export function sizeOfWholeDays(days: number): string {
	if (days < 1) return sizeOfWholeDayNames[0];
	return sizeOfWholeDayNames[days - 1] ?? sizes[sizes.length - 1].name;
}
