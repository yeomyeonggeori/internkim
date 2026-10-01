import { z } from 'zod';
import { dataRoomCategorySchema, dataRoomRoleSchema } from './model';

export const sharedDataRoomSchema = z.object({
	categories: z.array(z.object({ code: z.string(), parent: z.string().nullable(),
		name: z.string(), name_ko: z.string(), description: z.string(), slug: z.string() })),
	documents: z.array(z.object({ id: z.string(), title: z.string(), summary: z.string().nullable(),
		category_code: z.string(), document_date: z.string().nullable(), status: z.string().nullable() }))
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
  issuedAt: z.string(),
  categoryCode: z.string().nullable(),
  clearance: z.number().int(),
  domain: z.string().nullable(),
  date: z.string().nullable(),
  period: z.string().nullable(),
  status: z.string().nullable(),
  supersedes: z.string().nullable(),
  sha256: z.string().nullable(),
  tags: z.array(z.string()),
  storagePath: z.string().nullable(),
  published: companyDocumentPublishedSchema.nullable(),
});

export const companyDocumentListResultSchema = z.strictObject({
 count: z.number().int(), documents: z.array(companyDocumentResultSchema)
});

export const dataRoomShareSchema = z.strictObject({
 id: z.string(), roleCode: z.string(), audience: z.enum(['member', 'circle', 'email', 'public']),
 email: z.string().nullable(), memberID: z.string().nullable(), circleID: z.string().nullable(),
 acceptedAt: z.string().nullable(), expiresAt: z.string().nullable(), revokedAt: z.string().nullable(),
 canDownload: z.boolean()
});

export const dataRoomGetResultSchema = z.strictObject({
 categories: z.array(dataRoomCategorySchema), roles: z.array(dataRoomRoleSchema),
 shares: z.array(dataRoomShareSchema), canManage: z.boolean()
});
