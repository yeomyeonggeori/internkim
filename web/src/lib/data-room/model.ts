import { z } from 'zod';
import defaultTemplate from './template.json';

export const dataRoomCategorySchema = z.strictObject({
	code: z.string().regex(/^[A-Z]{1,2}$/),
	slug: z.string().min(1),
	name: z.string().min(1),
	nameKO: z.string().min(1),
	parent: z.string().nullable(),
	description: z.string(),
	choiceGroup: z.number().int().min(1).max(2).nullable().optional()
});

export const dataRoomRoleSchema = z.strictObject({
	code: z.string().min(1),
	name: z.string().min(1),
	nameKO: z.string(),
	readableCategories: z.array(z.string())
});

export type DataRoomCategory = z.infer<typeof dataRoomCategorySchema>;
export type DataRoomRole = z.infer<typeof dataRoomRoleSchema>;

export const dataRoomTemplate = z.strictObject({
	version: z.number().int(),
	categories: z.array(dataRoomCategorySchema),
	roles: z.array(dataRoomRoleSchema)
}).parse(defaultTemplate);

export function filingCategories(categories: DataRoomCategory[]): DataRoomCategory[] {
	return categories.filter((category) => category.parent !== null || category.code === 'X');
}

export function normalizeCategoryGrants(codes: string[], categories: DataRoomCategory[]): string[] | undefined {
	const uniqueCodes = [...new Set(codes)];
	if (uniqueCodes.some((code) => !categories.some((category) => category.code === code))) return undefined;
	return uniqueCodes.filter((code) => {
		const category = categories.find((candidate) => candidate.code === code);
		return category && (!category.parent || !uniqueCodes.includes(category.parent));
	});
}

export function categoryName(category: DataRoomCategory, locale: string): string {
	return locale === 'ko' ? category.nameKO : category.name;
}
