import { z } from 'zod';
import defaultTemplate from './template.json';

export const companyFieldKinds = ['profile', 'metric', 'record'] as const;
export const companyFieldKindSchema = z.enum(companyFieldKinds);
export type CompanyFieldKind = z.infer<typeof companyFieldKindSchema>;

export const dataRoomCategorySchema = z.strictObject({
	code: z.string().regex(/^[A-Z]{1,2}$/),
	slug: z.string().min(1),
	name: z.string().min(1),
	nameKO: z.string().min(1),
	parent: z.string().nullable(),
	description: z.string(),
	fields: z.partialRecord(companyFieldKindSchema, z.array(z.string().min(1))).optional(),
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
