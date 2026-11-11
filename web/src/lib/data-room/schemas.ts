import { z } from 'zod';
import { circleIDSchema, companyFieldKindSchema, dataRoomCategorySchema } from './model';
import { dataRoomCompanySchema } from './company';

export const sharedDocumentColumns = 'id,title,summary,category_code,document_date,status,storage_path';

export const sharedDataRoomSchema = z.object({
	categories: z.array(z.object({ code: z.string(), parent: z.string().nullable(),
		name: z.string(), name_ko: z.string(), description: z.string(), slug: z.string() })),
	documents: z.array(z.object({ id: z.string(), title: z.string(), summary: z.string().nullable(),
		category_code: z.string(), document_date: z.string().nullable(), status: z.string().nullable(),
		extension: z.string().nullable() })),
	company: dataRoomCompanySchema,
	viewer: z.string().nullable()
});

export const previewedDataRoomSchema = sharedDataRoomSchema.extend({
	canDownload: z.boolean()
});

export const sharedDataRoomLinkSchema = previewedDataRoomSchema.extend({
	expiresAt: z.string()
});

export const companyDocumentPublishedSchema = z.strictObject({
  at: z.string(),
  by: z.string().nullable(),
  from: z.string().nullable(),
});

export const companyDocumentResultSchema = z.strictObject({
  documentID: z.string(),
  documentNumber: z.string().nullable(),
  kind: z.string(),
  documentType: z.string(),
  title: z.string(),
  counterpart: z.string().nullable(),
  language: z.string().nullable(),
  filePath: z.string().nullable(),
  summary: z.string().nullable(),
  requesterID: z.string().nullable(),
  createdAt: z.string(),
  updatedAt: z.string(),
  categoryCode: z.string(),
  date: z.string().nullable(),
  period: z.string().nullable(),
  status: z.string().nullable(),
  supersedes: z.string().nullable(),
  sha256: z.string().nullable(),
  tags: z.array(z.string()),
  storagePath: z.string().nullable(),
  sizeBytes: z.number().int().nullable(),
  published: companyDocumentPublishedSchema.nullable(),
});

export const companyDocumentListResultSchema = z.strictObject({
 count: z.number().int(), documents: z.array(companyDocumentResultSchema)
});

export const dataRoomShareSchema = z.strictObject({
 id: z.string(), circleID: z.string(), audience: z.enum(['email', 'public']),
 email: z.string().nullable(),
 acceptedAt: z.string().nullable(), expiresAt: z.string().nullable(), revokedAt: z.string().nullable(),
 canDownload: z.boolean()
});

export const fieldNamesSchema = z.record(z.string().regex(/^[a-z]{2}$/), z.string().trim().min(1));

export const companyFieldSchema = z.strictObject({
 kind: companyFieldKindSchema.describe("profile: a company_info key, 'activity' or 'ownership'; metric: a company_metric key; record: a company_record category"),
 field: z.string(),
 categoryCode: z.string().nullable().describe('The category whose readers see it; null shows it only to readers of X'),
 name: z.string().nullable().describe("What the company calls it in English; null shows the product's name"),
 nameKO: z.string().nullable().describe("What the company calls it in Korean; null shows the product's name"),
 unit: z.string().nullable().describe('The unit of a metric counted in value; null for money and for every other kind')
});

export type CompanyFieldNameColumns = { name?: string | null; nameKO?: string | null };

export function fieldNamesOf(columns: CompanyFieldNameColumns): Record<string, string> | null {
 const names = Object.fromEntries(
  [['en', columns.name], ['ko', columns.nameKO]].flatMap(([language, name]) => (name?.trim() ? [[language, name.trim()]] : []))
 );
 return Object.keys(names).length > 0 ? names : null;
}

export const dataRoomGetResultSchema = z.strictObject({
 categories: z.array(dataRoomCategorySchema.omit({ fields: true })),
 fields: z.array(companyFieldSchema).describe('The company page fields placed in a category, named, or given a unit. A field absent here is shown only to readers of X, under the product name.'),
 shares: z.array(dataRoomShareSchema), canManage: z.boolean()
});

export const circleListResultSchema = z.strictObject({
 circles: z.array(z.strictObject({
  id: circleIDSchema.describe('The lowercase English name the company host gives its folder and group; it never changes'),
  name: z.string().describe("What people call it. A default circle's name is the product's, in the company's language"),
  isDefault: z.boolean().describe('Whether the product named this circle; a default circle cannot be renamed'),
  readableCategories: z.array(z.string()),
  memberIDs: z.array(z.string()),
  channelName: z.string().nullable().describe("The name of the circle's own private messenger channel, or null when it has none. Its members are exactly the circle's."),
  channelID: z.string().nullable().describe('The messenger channel the company computer opened for it, or null until it has')
 }))
});

export type ListedCircle = z.infer<typeof circleListResultSchema>['circles'][number];
