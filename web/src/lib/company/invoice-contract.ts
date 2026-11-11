import { z } from 'zod';

export const companyInvoiceSchema = z.object({
	id: z.string(),
	issuedAt: z.string(),
	description: z.string(),
	amount: z.number().int(),
	currency: z.string(),
	status: z.enum(['paid', 'partially_refunded', 'refunded', 'open', 'void']),
	receiptURL: z.string().nullable()
});

export type CompanyInvoice = z.infer<typeof companyInvoiceSchema>;

export const companyInvoicesSchema = z.object({ invoices: z.array(companyInvoiceSchema) });
