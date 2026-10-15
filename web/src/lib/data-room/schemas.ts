import { z } from 'zod';
import { circleIDSchema, dataRoomCategorySchema } from './model';
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

export const dataRoomGetResultSchema = z.strictObject({
 categories: z.array(dataRoomCategorySchema), shares: z.array(dataRoomShareSchema), canManage: z.boolean()
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
