import { z } from 'zod';

import {
  CapabilityAnsweredBy,
  CapabilityEstimatedLatency,
  CapabilityModelVisibility,
  CapabilitySideEffect,
} from './protocol';

import type { CapabilityToolDefinition } from './definition';

const companyMetricCurrencies = [
  'USD', 'KRW', 'EUR', 'JPY', 'GBP', 'CNY', 'HKD', 'SGD', 'AUD', 'CAD', 'CHF', 'INR',
] as const;

const dataRoomClearanceSchema = z.int().min(0).max(3);

const dataRoomDomainSchema = z.string().describe("Data room domain the document is filed under, e.g. 'finance', 'contracts', 'governance'. The domain decides who reads it: a domain is a folder and a clearance.");

const dataRoomSha256Schema = z.string().describe("Lowercase hex SHA-256 of the original file. Keys the object in the asset bucket and every file derived from it.");

const dataRoomDocumentFields = {
  clearance: dataRoomClearanceSchema.describe("Data room clearance the document is readable at: 0 public, 1 every member, 2 management, 3 representative and board. A member registers at their own clearance or below; the record refuses higher.").optional(),
  date: z.string().describe("The date the document speaks from, in YYYY-MM-DD format.").optional(),
  domain: dataRoomDomainSchema.optional(),
  period: z.string().describe("The period the document covers when there is one, e.g. '2025' or '2026-Q1'.").optional(),
  sha256: dataRoomSha256Schema.optional(),
  status: z.string().describe("'current', 'superseded' or 'draft'.").optional(),
  storagePath: z.string().describe("Where the original sits in the asset bucket, as company_document_upload answered it.").optional(),
  supersedesHint: z.string().describe("The document this one replaces: its id from a prior company_document_list or search result, its document number, or its exact CURRENT title. It must be filed in the same domain. Nothing is overwritten; the older document stays and this one names it.").optional(),
  tags: z.array(z.string()).describe("Short lowercase tags, e.g. ['audit', 'k-ifrs'].").optional(),
};

const companyDocumentListInputSchema = z.strictObject({
  clearance: dataRoomClearanceSchema.describe("Data room clearance to filter by, 0 to 3; only documents filed at exactly that clearance.").optional(),
  counterpart: z.string().describe("Counterpart name to filter by, e.g. 'ABC Trading'.").optional(),
  domain: dataRoomDomainSchema.optional(),
  query: z.string().describe("Keyword filter matched against title, summary, and counterpart.").optional(),
  type: z.string().describe("Document type slug to filter by, e.g. 'quote'. Leave empty for all types.").optional(),
});

const companyDocumentRegisterInputSchema = z.strictObject({
  ...dataRoomDocumentFields,
  counterpart: z.string().describe("Counterpart company or person name, e.g. 'ABC Trading'.").optional(),
  documentType: z.string().describe("Document type slug from the paperwork catalog, e.g. 'quote', 'service-agreement', 'employment-certificate'."),
  filePath: z.string().describe("Workspace path of the file if it already exists. For issued documents you can also set it later with company_document_update after saving.").optional(),
  kind: z.string().describe("'issued' for documents the company creates (default, gets a document number), 'received' for documents from counterparts, 'internal' for internal-only files.").optional(),
  language: z.string().describe("Document language, e.g. 'ko' or 'en'.").optional(),
  summary: z.string().describe("2-3 sentence summary of the document's key terms: parties, amounts, dates, obligations. Written so later questions can be answered without opening the file."),
  title: z.string().describe("Document title including the counterpart, e.g. 'ABC Trading onboarding consulting quote'."),
});

const companyDocumentSearchInputSchema = z.strictObject({
  limit: z.int().describe("Maximum documents to return. Defaults to 5.").optional(),
  query: z.string().describe("Natural-language question or topic, e.g. 'payment terms of the service agreement with ABC Trading'."),
});

const companyDocumentUpdateInputSchema = z.strictObject({
  ...dataRoomDocumentFields,
  counterpart: z.string().describe("Corrected counterpart. Omit to keep unchanged.").optional(),
  documentHint: z.string().describe("The document to update: its id from a prior company_document_list or search result, its document number, or its exact CURRENT title. Never the new title this call is about to set."),
  filePath: z.string().describe("New workspace path after the file was saved, moved, or renamed.").optional(),
  summary: z.string().describe("Replacement summary. Omit to keep unchanged.").optional(),
  title: z.string().describe("Corrected title. Omit to keep unchanged.").optional(),
});

const companyDocumentUploadInputSchema = z.strictObject({
  clearance: dataRoomClearanceSchema.describe("Data room clearance the file is stored at: 0 public, 1 every member, 2 management, 3 representative and board. The bucket refuses a clearance above the requester's own."),
  fileName: z.string().describe("Name of a file derived from the original, e.g. '01-summary.md' or 'thumbnail.png', stored beside it under the same hash. Omit for the original itself.").optional(),
  sha256: dataRoomSha256Schema,
});

const companyDocumentDownloadInputSchema = z.strictObject({
  documentHint: z.string().describe("The document whose original to fetch: its id from a prior company_document_list or search result, its document number, or its exact CURRENT title. Give this or storagePath.").optional(),
  fileName: z.string().describe("Name of a derived file stored beside the original, e.g. '01-summary.md'. Omit for the original itself.").optional(),
  storagePath: z.string().describe("The object's path in the asset bucket, as a document result's storagePath shows it. Give this or documentHint.").optional(),
});

const companyInfoGetInputSchema = z.strictObject({
  language: z.string().describe("Document language to resolve the profile for, e.g. 'ko' or 'en'. Defaults to 'ko'. The response's missingFields lists core fields still empty for this language.").optional(),
});

const companyInfoSetInputSchema = z.strictObject({
  address: z.string().describe("Registered head-office address.").optional(),
  bankAccount: z.string().describe("One-line bank account: bank, account number, holder. Use the 'en' slot for international wire details (SWIFT/IBAN).").optional(),
  brandName: z.string().describe("Service or brand name when it differs from the legal name.").optional(),
  capital: z.string().describe("Paid-in capital, written the way the company states it, e.g. '500,000,000 KRW'.").optional(),
  description: z.string().describe("One-line company description for proposals and IR material.").optional(),
  email: z.string().describe("Main company email address.").optional(),
  employeeCount: z.int().describe("Official employee headcount. Independent of platform member count.").optional(),
  fax: z.string().describe("Fax number.").optional(),
  fiscalYearEnd: z.string().describe("Fiscal year end month, e.g. 'December'.").optional(),
  foundedDate: z.string().describe("Founding date in YYYY-MM-DD format.").optional(),
  jurisdiction: z.string().describe("Jurisdiction of incorporation for contract preambles, e.g. 'the Republic of Korea' or 'the State of Delaware'.").optional(),
  language: z.string().describe("Language slot the localized values belong to, e.g. 'ko' or 'en'."),
  legalAttributes: z.string().describe("JSON object string of country-specific label-to-value pairs, e.g. {\"Business Registration Number\": \"123-45-67890\", \"Business Type\": \"Services\"}. Write each label in the language the country prints it in, because labels print on documents as-is.").optional(),
  name: z.string().describe("Legal company name as it is registered, including the form of incorporation.").optional(),
  officeAddress: z.string().describe("Working office address when it differs from the registered address.").optional(),
  phone: z.string().describe("Main company phone number.").optional(),
  representative: z.string().describe("Representative's name; use the romanized name for the 'en' slot.").optional(),
  representativeTitle: z.string().describe("Representative's title. Defaults to the one customary in the language asked for.").optional(),
  slogan: z.string().describe("Company slogan for letterheads and introductions.").optional(),
  website: z.string().describe("Company website URL.").optional(),
});

const companyMetricListInputSchema = z.strictObject({
  fromYear: z.int().describe("Earliest year to include.").optional(),
  metric: z.string().describe("Metric key to filter by, e.g. 'annualRevenue'. Leave empty for all metrics.").optional(),
  toYear: z.int().describe("Latest year to include.").optional(),
});

const companyMetricRecordInputSchema = z.strictObject({
  currency: z.enum(companyMetricCurrencies).describe("ISO currency of a monetary value. Use currency instead of unit for money.").optional(),
  metric: z.string().describe("Metric key in lowerCamelCase, e.g. 'annualRevenue', 'operatingProfit', 'mau', 'employees'. Reuse the same key across periods."),
  month: z.int().describe("Month 1-12 for a monthly value. Leave out for annual or quarterly values; never combine with quarter.").optional(),
  note: z.string().describe("Source or context note, e.g. 'from the audited financial statements'.").optional(),
  quarter: z.int().describe("Quarter 1-4 for a quarterly value. Leave out for annual or monthly values; never combine with month.").optional(),
  unit: z.string().describe("Unit for a non-monetary value, e.g. 'people', 'orders', 'sites'. Do not combine with currency.").optional(),
  value: z.number().describe("Numeric value, e.g. 1200000000 for 1.2 billion. Use the raw number, not a formatted string."),
  valueUSD: z.number().describe("Stable USD equivalent for a non-USD monetary value. Required when currency is not USD; omitted for non-monetary values.").optional(),
  year: z.int().describe("Four-digit year the value belongs to, e.g. 2025."),
});

const companyRecordAddInputSchema = z.strictObject({
  attributes: z.string().describe("JSON object string of structured details, e.g. {\"round\": \"Seed\", \"amount\": \"2,000,000 USD\", \"investors\": \"ABC Ventures\"}.").optional(),
  category: z.string().describe("Record category: 'history', 'funding', 'product', 'certification', 'ip', 'award', 'reference', 'grant', or another short kebab-case label."),
  date: z.string().describe("Date of the event in YYYY-MM-DD or YYYY-MM format. Used to sort the company timeline.").optional(),
  detail: z.string().describe("One-to-three sentence description.").optional(),
  title: z.string().describe("Short title, e.g. 'Seed round closed' or 'Product launched'."),
});

const companyRecordDeleteInputSchema = z.strictObject({
  recordHint: z.string().describe("The record to delete: its id from a prior company_record_list result, or its exact CURRENT title. Never invent an id."),
});

const companyRecordListInputSchema = z.strictObject({
  category: z.string().describe("Category to filter by, e.g. 'funding' or 'history'. Leave empty for all.").optional(),
  query: z.string().describe("Keyword filter matched against title, detail, and attributes.").optional(),
});

const companyRecordUpdateInputSchema = z.strictObject({
  attributes: z.string().describe("JSON object string replacing the stored attributes. Omit to keep unchanged.").optional(),
  category: z.string().describe("New category. Omit to keep unchanged.").optional(),
  date: z.string().describe("New date. Omit to keep unchanged.").optional(),
  detail: z.string().describe("New detail text. Omit to keep unchanged.").optional(),
  recordHint: z.string().describe("The record to update: its id from a prior company_record_list result, or its exact CURRENT title. Never the new title this call is about to set."),
  title: z.string().describe("New title. Omit to keep unchanged.").optional(),
});

export const companyProfileLegalAttributeSchema = z.strictObject({
  label: z.string(),
  value: z.string(),
});

export const companyProfileResultSchema = z.strictObject({
  language: z.string(),
  name: z.string(),
  brandName: z.string(),
  slogan: z.string(),
  description: z.string(),
  representative: z.string(),
  representativeTitle: z.string(),
  address: z.string(),
  officeAddress: z.string(),
  jurisdiction: z.string(),
  bankAccount: z.string(),
  legalAttributes: z.array(companyProfileLegalAttributeSchema),
  foundedDate: z.string(),
  capital: z.string(),
  fiscalYearEnd: z.string(),
  employeeCount: z.number().int(),
  phone: z.string(),
  fax: z.string(),
  email: z.string(),
  website: z.string(),
  missingFields: z.array(z.string()),
  updatedAt: z.string(),
});

export const companyMetricResultSchema = z.strictObject({
  metricID: z.string(),
  metric: z.string(),
  year: z.number().int(),
  quarter: z.number().int(),
  month: z.number().int(),
  value: z.number(),
  currency: z.string().nullable(),
  valueUSD: z.number().nullable(),
  unit: z.string().nullable(),
  note: z.string().nullable(),
  updatedAt: z.string(),
});

export const companyMetricListResultSchema = z.strictObject({
  count: z.number().int(),
  metrics: z.array(companyMetricResultSchema),
});

export const companyRecordAttributeSchema = z.strictObject({
  label: z.string(),
  value: z.string(),
});

export const companyRecordResultSchema = z.strictObject({
  recordID: z.string(),
  category: z.string(),
  date: z.string().nullable(),
  title: z.string(),
  detail: z.string().nullable(),
  attributes: z.array(companyRecordAttributeSchema),
  updatedAt: z.string(),
});

export const companyRecordListResultSchema = z.strictObject({
  count: z.number().int(),
  records: z.array(companyRecordResultSchema),
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

export const companyDocumentUploadResultSchema = z.strictObject({
  storagePath: z.string(),
  uploadURL: z.string(),
});

export const companyDocumentDownloadResultSchema = z.strictObject({
  storagePath: z.string(),
  downloadURL: z.string(),
});

export const companyDocumentRegisteredResultSchema = z.strictObject({
  ...companyDocumentResultSchema.shape,
  storageDirectory: z.string(),
});

export const companyDocumentListResultSchema = z.strictObject({
  count: z.number().int(),
  documents: z.array(companyDocumentResultSchema),
});

export const companyToolDefinitions: CapabilityToolDefinition[] = [
  {
    name: "company_document_list",
    namespace: "company",
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: "workspace_company",
    policyResource: "tool:company_document_list",
    description: "List registered company documents newest first, with their numbers, counterparts, file paths, summaries, and where each sits in the data room. Filter by type, counterpart, domain, clearance, or keyword. Only documents at or below the requester's clearance are listed. Use to answer 'what quotes did we send to X'.",
    version: "2",
    estimatedLatency: CapabilityEstimatedLatency.Low,
    modelVisibility: CapabilityModelVisibility.Hidden,
    inputSchema: companyDocumentListInputSchema,
    result: { schema: companyDocumentListResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Read,
  },
  {
    name: "company_document_register",
    namespace: "company",
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: "workspace_company",
    policyResource: "tool:company_document_register",
    description: "Register a company document in the document ledger and, for kind=issued, receive the official document number to print in the document plus the storage directory to save the final file in. Call BEFORE rendering an official document so the number appears in it. Always include a 2-3 sentence summary of the document's key terms (parties, amounts, dates) so later questions can be answered without re-reading the file. A document filed in the data room also names its domain, clearance, date, hash and the storagePath company_document_upload answered; one that replaces an older document names it with supersedesHint instead of editing it.",
    version: "2",
    estimatedLatency: CapabilityEstimatedLatency.Low,
    modelVisibility: CapabilityModelVisibility.Hidden,
    inputSchema: companyDocumentRegisterInputSchema,
    result: { schema: companyDocumentRegisteredResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
    completionEvidence: { mode: "success", action: "write_company", targetKind: "company" },
  },
  {
    name: "company_document_search",
    namespace: "company",
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: "workspace_company",
    policyResource: "tool:company_document_search",
    description: "Search registered company documents by a natural-language question ('the terms of the agreement with ABC'). Matches the question against each document's title, counterpart, summary and type, best match first. Answer from the summary first and open the file only when detail is needed.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    modelVisibility: CapabilityModelVisibility.Hidden,
    inputSchema: companyDocumentSearchInputSchema,
    result: { schema: companyDocumentListResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Read,
  },
  {
    name: "company_document_update",
    namespace: "company",
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: "workspace_company",
    policyResource: "tool:company_document_update",
    description: "Update a registered document's file path, title, counterpart, summary, or where it sits in the data room: its domain, clearance, date, period, status, tags, hash, storage path, and the document it supersedes. Name the document with documentHint from a prior list or search result. Use when a file was moved or renamed so the ledger keeps tracking it, or when a submission is raised into its domain. Raising a clearance is refused above the requester's own.",
    version: "2",
    estimatedLatency: CapabilityEstimatedLatency.Low,
    modelVisibility: CapabilityModelVisibility.Hidden,
    inputSchema: companyDocumentUpdateInputSchema,
    result: { schema: companyDocumentResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
  },
  {
    name: "company_document_upload",
    namespace: "company",
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: "workspace_company",
    policyResource: "tool:company_document_upload",
    description: "Ask for a place in the data room to put one file. Answers the storagePath the file will sit at, keyed by its clearance and SHA-256, and a signed URL to PUT the bytes to; the record's own policy decides whether the requester may write at that clearance, so a refusal here is the clearance rule. Upload the original first, then each derived file with its fileName under the same hash, then register or update the document with the storagePath. The URL is good for two hours.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Low,
    modelVisibility: CapabilityModelVisibility.Hidden,
    inputSchema: companyDocumentUploadInputSchema,
    result: { schema: companyDocumentUploadResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
  },
  {
    name: "company_document_download",
    namespace: "company",
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: "workspace_company",
    policyResource: "tool:company_document_download",
    description: "Fetch a document's original, or one of the files derived from it, out of the data room. Name the document with documentHint, or give a storagePath from a document result, and add fileName for a derived file. Answers a signed URL good for ten minutes; a document above the requester's clearance does not exist for them, so it is refused as not found. Read the sidecar and the summary first and fetch the original only when they cannot answer; an original goes to the requester themselves and passing it on is their own act.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Low,
    modelVisibility: CapabilityModelVisibility.Hidden,
    inputSchema: companyDocumentDownloadInputSchema,
    result: { schema: companyDocumentDownloadResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Read,
  },
  {
    name: "company_info_get",
    namespace: "company",
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: "workspace_company",
    policyResource: "tool:company_info_get",
    description: "Read the company master profile (name, representative, address, contact, bank account, country-specific legal attributes such as a business registration number). Pass language ('ko' or 'en') to get the view for that document language plus missingFields listing empty core fields. Call this before creating any company letterhead document; if missingFields is empty, never ask the user for company info again.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Low,
    modelVisibility: CapabilityModelVisibility.Hidden,
    inputSchema: companyInfoGetInputSchema,
    result: { schema: companyProfileResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Read,
  },
  {
    name: "company_info_set",
    namespace: "company",
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: "workspace_company",
    policyResource: "tool:company_info_set",
    description: "Save or update the company master profile. Partial update: only provided fields are written, into the given language's slot for localized fields. Use after the user supplies company details, or when they report one has changed. Put country-specific identifiers (a business registration number, a corporate registration number, an industry classification, an EIN …) into legalAttributes as a label-to-value JSON object string.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Low,
    modelVisibility: CapabilityModelVisibility.Hidden,
    inputSchema: companyInfoSetInputSchema,
    result: { schema: companyProfileResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
    completionEvidence: { mode: "success", action: "write_company", targetKind: "company" },
  },
  {
    name: "company_metric_list",
    namespace: "company",
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: "workspace_company",
    policyResource: "tool:company_metric_list",
    description: "List recorded company metrics sorted by period. Filter by metric key and year range. Use for IR decks, business plans, and grant applications that need revenue/headcount/usage time series.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Low,
    modelVisibility: CapabilityModelVisibility.Hidden,
    inputSchema: companyMetricListInputSchema,
    result: { schema: companyMetricListResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Read,
  },
  {
    name: "company_metric_record",
    namespace: "company",
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: "workspace_company",
    policyResource: "tool:company_metric_record",
    description: "Record or correct one company metric value for a period — annual revenue, operating profit, MAU, employee count, GMV and similar time-series numbers. Monetary values use an allowed currency plus a stable USD equivalent; non-monetary values use unit. Add quarter (1-4) OR month (1-12) for sub-annual periods, never both. Same call overwrites the same period.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Low,
    modelVisibility: CapabilityModelVisibility.Hidden,
    inputSchema: companyMetricRecordInputSchema,
    result: { schema: companyMetricResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
    completionEvidence: { mode: "success", action: "write_company", targetKind: "company" },
  },
  {
    name: "company_record_add",
    namespace: "company",
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: "workspace_company",
    policyResource: "tool:company_record_add",
    description: "Add one company history/asset record: milestones, funding rounds, products, patents, certifications, awards, client references, government grants. Set category, date, title; put structured details (amount, investors, round …) into attributes as a label-to-value JSON object string.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Low,
    modelVisibility: CapabilityModelVisibility.Hidden,
    inputSchema: companyRecordAddInputSchema,
    result: { schema: companyRecordResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
    completionEvidence: { mode: "success", action: "write_company", targetKind: "company" },
  },
  {
    name: "company_record_delete",
    namespace: "company",
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: "workspace_company",
    policyResource: "tool:company_record_delete",
    description: "Delete a company record named by recordHint from a prior company_record_list result. Use only when the user asks to remove a wrong entry.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Low,
    modelVisibility: CapabilityModelVisibility.Hidden,
    inputSchema: companyRecordDeleteInputSchema,
    result: { schema: companyRecordResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Destructive,
    requiresApproval: true,
  },
  {
    name: "company_record_list",
    namespace: "company",
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: "workspace_company",
    policyResource: "tool:company_record_list",
    description: "List company history/asset records, newest first. Filter by category (history, funding, product, certification, ip, award, reference, grant …) or keyword query. Use to build company timelines, funding tables, and product overviews.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Low,
    modelVisibility: CapabilityModelVisibility.Hidden,
    inputSchema: companyRecordListInputSchema,
    result: { schema: companyRecordListResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Read,
  },
  {
    name: "company_record_update",
    namespace: "company",
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: "workspace_company",
    policyResource: "tool:company_record_update",
    description: "Update fields on an existing company record named by recordHint from a prior company_record_list result. Only provided fields change.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Low,
    modelVisibility: CapabilityModelVisibility.Hidden,
    inputSchema: companyRecordUpdateInputSchema,
    result: { schema: companyRecordResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
  },
];

export type CompanyInfoGetInput = z.infer<typeof companyInfoGetInputSchema>;
export type CompanyInfoSetInput = z.infer<typeof companyInfoSetInputSchema>;
export type CompanyProfileLegalAttribute = z.infer<typeof companyProfileLegalAttributeSchema>;
export type CompanyProfileResult = z.infer<typeof companyProfileResultSchema>;
export type CompanyMetricResult = z.infer<typeof companyMetricResultSchema>;
export type CompanyMetricListResult = z.infer<typeof companyMetricListResultSchema>;
export type CompanyRecordAttribute = z.infer<typeof companyRecordAttributeSchema>;
export type CompanyRecordResult = z.infer<typeof companyRecordResultSchema>;
export type CompanyRecordListResult = z.infer<typeof companyRecordListResultSchema>;
export type CompanyDocumentResult = z.infer<typeof companyDocumentResultSchema>;
export type CompanyDocumentRegisteredResult = z.infer<typeof companyDocumentRegisteredResultSchema>;
export type CompanyDocumentListResult = z.infer<typeof companyDocumentListResultSchema>;
export type CompanyDocumentPublished = z.infer<typeof companyDocumentPublishedSchema>;
export type CompanyDocumentUploadResult = z.infer<typeof companyDocumentUploadResultSchema>;
export type CompanyDocumentDownloadResult = z.infer<typeof companyDocumentDownloadResultSchema>;
