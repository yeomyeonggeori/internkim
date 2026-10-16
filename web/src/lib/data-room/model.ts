import { z } from 'zod';
import defaultTemplate from './template.json';

export const dataRoomCategorySchema = z.strictObject({
	code: z.string().regex(/^[A-Z]{1,2}$/),
	slug: z.string().min(1),
	name: z.string().min(1),
	nameKO: z.string().min(1),
	parent: z.string().nullable(),
	description: z.string(),
	fields: z.array(z.string().min(1)).describe("What the company page shows to readers of this category: company_info keys, company_metric keys, company_record categories and 'activity' (attendance and work with names masked), e.g. ['capital'] or ['annualRevenue']. A field belongs to one category; one no category names is shown only to readers of X. Move one with dataroom_field_update.").optional(),
	choiceGroup: z.number().int().min(1).max(2).nullable().optional()
});

export const circleIDSchema = z.string().regex(/^[a-z][a-z0-9-]*$/);

const templateCircleSchema = z.strictObject({
	id: circleIDSchema,
	name: z.string().min(1),
	nameKO: z.string().min(1),
	readableCategories: z.array(z.string())
});

export type DataRoomCategory = z.infer<typeof dataRoomCategorySchema>;
export type TemplateCircle = z.infer<typeof templateCircleSchema>;

export const dataRoomTemplate = z.strictObject({
	version: z.number().int(),
	categories: z.array(dataRoomCategorySchema),
	circles: z.array(templateCircleSchema)
}).parse(defaultTemplate);

export function defaultCircleName(circleID: string, locale: string): string | undefined {
	const circle = dataRoomTemplate.circles.find((candidate) => candidate.id === circleID);
	if (!circle) return undefined;
	return locale === 'ko' ? circle.nameKO : circle.name;
}

export function circleName(circle: { id: string; name: string | null }, locale: string): string {
	return defaultCircleName(circle.id, locale) ?? circle.name ?? circle.id;
}

export function filingCategories(categories: DataRoomCategory[]): DataRoomCategory[] {
	const parentCodes = new Set(categories.map((category) => category.parent));
	return categories.filter((category) => !parentCodes.has(category.code));
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
