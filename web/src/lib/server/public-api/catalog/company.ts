import { z } from 'zod';
import { companyDocumentPublishedSchema, companyDocumentResultSchema, companyDocumentListResultSchema } from '$lib/data-room/schemas';
export { companyDocumentPublishedSchema, companyDocumentResultSchema, companyDocumentListResultSchema } from '$lib/data-room/schemas';

import {
  CapabilityAnsweredBy,
  CapabilityEstimatedLatency,
  CapabilityModelVisibility,
  CapabilitySideEffect,
  ResourceEffectIdentity,
  recallSourceInputSchema,
  recallSourceResultSchema,
} from './protocol';

import { ResourceMutationEffect, type CapabilityToolDefinition } from './definition';
import { imageFieldOf, serviceFileImageExtensions, serviceFileNames, serviceFiles } from '$lib/data-room/service-files';
import { moneySchema } from '$lib/money';
import { momentSchema } from '$lib/moment';
import { companyPictureImage } from '$lib/company/company-picture';




const dataRoomSha256Schema = z.string().describe("Lowercase hex SHA-256 of the original file. Keys the object in the asset bucket and every file derived from it.");

const dataRoomDocumentFields = {
  categoryCode: z.string().regex(/^[A-Z]{1,2}$/).describe("Exact leaf category code from dataroom_get, such as FS, FP, or a parent with no children. Parents with children cannot hold new filings. Choose by business function; use X when context is insufficient.").optional(),
  date: z.string().describe("The date the document speaks from, in YYYY-MM-DD format.").optional(),
  period: z.string().describe("The period the document covers when there is one, e.g. '2025' or '2026-Q1'.").optional(),
  sha256: dataRoomSha256Schema.optional(),
  status: z.string().describe("'current', 'superseded' or 'draft'.").optional(),
  supersedesHint: z.string().describe("The document this one replaces: its id from a prior company_document_list or search result, its document number, or its exact CURRENT title. It must be filed in the same category. Nothing is overwritten; the older document stays and this one names it.").optional(),
  tags: z.array(z.string()).describe("Short lowercase tags, e.g. ['audit', 'k-ifrs'].").optional(),
};

const companyDocumentListInputSchema = z.strictObject({
  categoryCode: z.string().regex(/^[A-Z]{1,2}$/).describe('Exact parent or intermediate category code; a parent includes its children.').optional(),
  counterpart: z.string().describe("Counterpart name to filter by, e.g. 'ABC Trading'.").optional(),
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

const companyDocumentRegisterInputIntentSchema = companyDocumentRegisterInputSchema.partial();

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

const companyDocumentUpdateInputIntentSchema = companyDocumentUpdateInputSchema.omit({ documentHint: true }).partial();

const companyDocumentUploadInputSchema = z.strictObject({
  documentHint: z.string().describe("The registered document the file belongs to: its id from company_document_register or a prior list or search result, its document number, or its exact CURRENT title."),
  originalFileName: z.string().describe("The original file's own name with its extension, e.g. '2025 audit report.pdf'. Give this or fileName.").optional(),
  fileName: z.string().describe("Name of a file derived from the original, e.g. 'content.txt' or 'thumbnail.png'. Give this or originalFileName.").optional(),
});

const companyDocumentUploadInputIntentSchema = companyDocumentUploadInputSchema.partial();

const companyDocumentDownloadInputSchema = z.strictObject({
  documentHint: z.string().describe("The document whose file to fetch: its id from a prior company_document_list or search result, its document number, or its exact CURRENT title."),
  fileName: z.string().describe("Name of a file derived from the original, e.g. 'content.txt'. Omit for the original itself.").optional(),
});

const companyInfoGetInputSchema = z.strictObject({
  language: z.string().describe("Document language to resolve the profile for, e.g. 'ko' or 'en'. Omit it to get the company's own language, which the response's language names. The response's missingFields lists core fields still empty for this language.").optional(),
});

const serviceFileImageFields = Object.fromEntries(serviceFiles.map((file) => [
  imageFieldOf(file.name),
  z.string().describe(`${file.title}: the storagePath company_image_upload answered for image '${file.name}', once the image was PUT to its uploadURL. It is kept as a new version beside the older ones, and forms print the version with the latest date.`).optional(),
]));

const serviceFileImageFieldNames = serviceFileNames.map(imageFieldOf).join(' or ');

const companyInfoSetInputSchema = z.strictObject({
  address: z.string().describe("Registered head-office address.").optional(),
  bankAccount: z.string().describe("One-line bank account: bank, account number, holder. Use the 'en' slot for international wire details (SWIFT/IBAN).").optional(),
  brandName: z.string().describe("Service or brand name when it differs from the legal name.").optional(),
  capital: moneySchema.describe("Paid-in capital as an amount and its currency, e.g. {\"amount\": 500000000, \"currency\": \"KRW\"}.").optional(),
  description: z.string().describe("One-line company description for proposals and IR material.").optional(),
  email: z.string().describe("Main company email address.").optional(),
  fax: z.string().describe("Fax number.").optional(),
  fiscalYearEnd: z.string().describe("Fiscal year end month, e.g. 'December'.").optional(),
  foundedDate: z.string().describe("Founding date in YYYY-MM-DD format.").optional(),
  jurisdiction: z.string().describe("Jurisdiction of incorporation for contract preambles, e.g. 'the Republic of Korea' or 'the State of Delaware'.").optional(),
  language: z.string().describe("Language slot the localized values belong to, e.g. 'ko' or 'en'."),
  legalAttributes: z.string().describe("JSON object string of country-specific label-to-value pairs, e.g. {\"Business Registration Number\": \"123-45-67890\", \"Business Type\": \"Services\"}. Write each label in the language the country prints it in, because labels print on documents as-is.").optional(),
  name: z.string().describe("Legal company name as it is registered, including the form of incorporation.").optional(),
  officeAddress: z.string().describe("Working office address when it differs from the registered address.").optional(),
  phone: z.string().describe("Main company phone number.").optional(),
  representativePersonHint: z.string().describe("The person who represents the company in law: their personID from person_list, or their name or email as written. Their directory name and job title print as the representative. An empty string clears it.").optional(),
  ...serviceFileImageFields,
  slogan: z.string().describe("Company slogan for letterheads and introductions.").optional(),
  website: z.string().describe("Company website URL.").optional(),
});

const companyInfoSetInputIntentSchema = companyInfoSetInputSchema.partial();

const companyImageUploadInputSchema = z.strictObject({
  image: z.enum([...serviceFileNames, companyPictureImage]).describe(`Which company image the file is: ${serviceFiles.map((file) => `'${file.name}' for ${file.purpose}`).join(', ')}, or '${companyPictureImage}' for the picture the app shows beside the company's name.`),
  fileName: z.string().describe(`The image file's own name with its extension: ${serviceFileImageExtensions.map((extension) => `.${extension}`).join(', ')}.`),
  date: z.string().describe(`The day the image took effect, in YYYY-MM-DD format. Defaults to today. A '${companyPictureImage}' has no versions, so it reads no date.`).optional(),
});

const companyImageUploadInputIntentSchema = companyImageUploadInputSchema.partial();

const companyMetricListInputSchema = z.strictObject({
  fromYear: z.int().describe("Earliest year to include.").optional(),
  metric: z.string().describe("Metric key to filter by, e.g. 'annualRevenue'. Leave empty for all metrics.").optional(),
  toYear: z.int().describe("Latest year to include.").optional(),
});

const companyMetricRecordInputSchema = z.strictObject({
  amount: moneySchema.describe("The value when the metric is money, e.g. {\"amount\": 1200000000, \"currency\": \"KRW\"}. Give amount or value, never both.").optional(),
  metric: z.string().describe("Metric key in lowerCamelCase, e.g. 'annualRevenue', 'operatingProfit', 'mau'. Reuse the same key across periods."),
  note: z.string().describe("Source or context note, e.g. 'from the audited financial statements'.").optional(),
  unit: z.string().describe("Unit of a value that is not money, e.g. 'people', 'orders', 'sites'.").optional(),
  usdEquivalent: z.number().describe("The amount in US dollars at the time it is recorded, kept so metrics compare across currencies. Required when amount is not in USD.").optional(),
  value: z.number().describe("The value when the metric is not money, e.g. 26400 for MAU. Use the raw number, not a formatted string.").optional(),
  period: momentSchema.describe("The period the value covers: YYYY for a year, YYYY-H1 or YYYY-H2 for a half, YYYY-Q1 to YYYY-Q4 for a quarter, YYYY-MM for a month, YYYY-MM-DD for a count taken on one day. A value recorded before its period ends is read as what the company expects for it; recording again once the period is over replaces it with the result."),
});

const companyMetricRecordInputIntentSchema = companyMetricRecordInputSchema.partial();

const companyRecordStatusSchema = z.enum(["in_progress", "completed"]).describe("'in_progress' while it is still happening, such as a funding round being raised (its amount is the target); 'completed' once it happened. Defaults to completed.");

const companyRecordAddInputSchema = z.strictObject({
  amount: moneySchema.describe("The money the event is about, e.g. a funding round or a grant: {\"amount\": 2000000, \"currency\": \"USD\"}.").optional(),
  attributes: z.string().describe("JSON object string of other structured details, e.g. {\"round\": \"Seed\", \"investors\": \"ABC Ventures\"}. The event's own money goes in amount; any other sum, such as a valuation, is money too: {\"preMoney\": {\"amount\": 30000000000, \"currency\": \"KRW\"}}.").optional(),
  category: z.string().describe("Record category: 'history', 'funding', 'product', 'certification', 'ip', 'award', 'reference', 'grant', or another short kebab-case label."),
  date: momentSchema.describe("When it happened, as precisely as known: YYYY-MM-DD, YYYY-MM, YYYY-Q1 to YYYY-Q4 for a quarter, YYYY-H1 or YYYY-H2 for a half, or YYYY. Sorts the company timeline. For something still in progress, when it is expected to finish.").optional(),
  detail: z.string().describe("One-to-three sentence description.").optional(),
  status: companyRecordStatusSchema.optional(),
  title: z.string().describe("Short title, e.g. 'Seed round closed' or 'Product launched'."),
});

const companyRecordAddInputIntentSchema = companyRecordAddInputSchema.partial();

const companyRecordDeleteInputSchema = z.strictObject({
  recordHint: z.string().describe("The record to delete: its id from a prior company_record_list result, or its exact CURRENT title. Never invent an id."),
});

const companyRecordDeleteInputIntentSchema = z.strictObject({});

const companyRecordListInputSchema = z.strictObject({
  category: z.string().describe("Category to filter by, e.g. 'funding' or 'history'. Leave empty for all.").optional(),
  query: z.string().describe("Keyword filter matched against title, detail, and attributes.").optional(),
});

const companyRecordUpdateInputSchema = z.strictObject({
  amount: moneySchema.describe("New amount. Omit to keep unchanged.").optional(),
  attributes: z.string().describe("JSON object string replacing the stored attributes. Omit to keep unchanged.").optional(),
  category: z.string().describe("New category. Omit to keep unchanged.").optional(),
  date: momentSchema.describe("New date: YYYY-MM-DD, YYYY-MM, YYYY-Q1 to YYYY-Q4, YYYY-H1 or YYYY-H2, or YYYY. Omit to keep unchanged.").optional(),
  detail: z.string().describe("New detail text. An empty string clears it. Omit to keep unchanged.").optional(),
  recordHint: z.string().describe("The record to update: its id from a prior company_record_list result, or its exact CURRENT title. Never the new title this call is about to set."),
  status: companyRecordStatusSchema.describe("New status, e.g. 'completed' when a round closes. Omit to keep unchanged.").optional(),
  title: z.string().describe("New title. Omit to keep unchanged.").optional(),
});

const companyRecordUpdateInputIntentSchema = companyRecordUpdateInputSchema.omit({ recordHint: true }).partial();

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
  representativePersonID: z.string().nullable(),
  address: z.string(),
  officeAddress: z.string(),
  jurisdiction: z.string(),
  bankAccount: z.string(),
  legalAttributes: z.array(companyProfileLegalAttributeSchema),
  foundedDate: z.string(),
  capital: moneySchema.nullable(),
  fiscalYearEnd: z.string(),
  employeeCount: z.number().int().describe("How many people work here now, counted from the organization: active human members whose hire date has come. Nobody writes it."),
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
  period: momentSchema,
  amount: moneySchema.nullable(),
  usdEquivalent: z.number().nullable(),
  value: z.number().nullable(),
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
  status: companyRecordStatusSchema,
  amount: moneySchema.nullable(),
  attributes: z.array(companyRecordAttributeSchema),
  updatedAt: z.string(),
});

export const companyRecordListResultSchema = z.strictObject({
  count: z.number().int(),
  records: z.array(companyRecordResultSchema),
});


export const companyDocumentUploadResultSchema = z.strictObject({
  storagePath: z.string(),
  uploadURL: z.string(),
});

export const companyImageUploadResultSchema = z.strictObject({
  storagePath: z.string(),
  uploadURL: z.string(),
});

export const companyDocumentDownloadResultSchema = z.strictObject({
  storagePath: z.string(),
  downloadURL: z.string(),
});


const equityHolderKindSchema = z.enum(["founder", "employee", "investor", "option_pool", "other"]);

const companyEquityAddInputSchema = z.strictObject({
  effectiveDate: z.string().describe("The day the change took effect, YYYY-MM-DD. To start a ledger from today's register, date every opening line on the day of that register."),
  fundingRoundHint: z.string().describe("The funding round that issued these shares: a funding record's id from company_record_list, or its exact CURRENT title. Leave out for founders' shares, grants and transfers.").optional(),
  holder: z.string().describe("The holder exactly as the shareholder register names them, e.g. 'Sample Kim' or 'Sample Ventures Fund I'."),
  holderKind: equityHolderKindSchema.describe("Who the holder is: 'founder', 'employee', 'investor', 'option_pool' for options reserved but not yet granted, or 'other'."),
  note: z.string().describe("Where the line comes from, e.g. 'shareholder register of 2026-06-30'.").optional(),
  securityClass: z.string().describe("What is held, as the company calls it, e.g. 'common', 'RCPS', 'Series A preferred', 'options'. Defaults to common.").optional(),
  shares: z.number().describe("Shares received (positive) or given up (negative); options count as the shares they convert into. Never zero."),
});

const companyEquityAddInputIntentSchema = companyEquityAddInputSchema.partial();

const companyEquityListInputSchema = z.strictObject({
  asOfDate: z.string().describe("Sum the ledger up to this day, YYYY-MM-DD. Leave out for today.").optional(),
  holder: z.string().describe("Only the lines of holders whose name contains this text.").optional(),
});

const companyEquityDeleteInputSchema = z.strictObject({
  entryHint: z.string().describe("The ledger line to delete: its entryID from a prior company_equity_list result. Never invent an id."),
});

const companyEquityDeleteInputIntentSchema = z.strictObject({});

export const companyEquityEntryResultSchema = z.strictObject({
  entryID: z.string(),
  effectiveDate: z.string(),
  holder: z.string(),
  holderKind: equityHolderKindSchema,
  securityClass: z.string(),
  shares: z.number(),
  fundingRecordID: z.string().nullable(),
  note: z.string().nullable(),
});

export const companyEquityListResultSchema = z.strictObject({
  asOfDate: z.string(),
  totalShares: z.number(),
  holdings: z.array(z.strictObject({
    holder: z.string(),
    holderKind: equityHolderKindSchema,
    securityClass: z.string(),
    shares: z.number(),
    percent: z.number(),
  })),
  entries: z.array(companyEquityEntryResultSchema),
});

export type CompanyEquityEntryResult = z.infer<typeof companyEquityEntryResultSchema>;
export type CompanyEquityListResult = z.infer<typeof companyEquityListResultSchema>;

export const companyToolDefinitions: CapabilityToolDefinition[] = [
  {
    name: "company_document_list",
    namespace: "company",
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: "workspace_company",
    policyResource: "tool:company_document_list",
    description: "List registered company documents newest first, with their numbers, counterparts, file paths, summaries, and where each sits in the data room. Filter by category, type, counterpart, or keyword. Category grants decide visibility. Use to answer 'what quotes did we send to X'.",
    version: "2",
    estimatedLatency: CapabilityEstimatedLatency.Low,
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
    description: "Register a company document in the document ledger and, for kind=issued, receive the official document number to print in the document. Call BEFORE rendering an official document so the number appears in it. Always include a 2-3 sentence summary of the document's key terms (parties, amounts, dates) so later questions can be answered without re-reading the file. A document filed in the data room names its categoryCode, date and hash, and its files are uploaded with company_document_upload once it is registered; one that replaces an older document names it with supersedesHint instead of editing it.",
    version: "2",
    estimatedLatency: CapabilityEstimatedLatency.Low,
    inputSchema: companyDocumentRegisterInputSchema,
    inputIntentSchema: companyDocumentRegisterInputIntentSchema,
    result: { schema: companyDocumentResultSchema, effects: [{ objectType: "company_document", effect: ResourceMutationEffect.Created, resultField: "documentID", effectIdentity: ResourceEffectIdentity.ID }] },
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
    inputSchema: companyDocumentSearchInputSchema,
    result: { schema: companyDocumentListResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Read,
  },
  {
    name: "company_document_recall",
    namespace: "company",
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: "workspace_company",
    policyResource: "tool:company_document_recall",
    description: "Recall the data room documents a request names by title or counterpart, or whose summary holds what it asks, one sentence each. The agent asks this beside its own memory whenever it recalls, so what the requester may read now arrives without being copied into memory.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Low,
    modelVisibility: CapabilityModelVisibility.Hidden,
    recallSource: true,
    inputSchema: recallSourceInputSchema,
    result: { schema: recallSourceResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Read,
  },
  {
    name: "company_document_update",
    namespace: "company",
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: "workspace_company",
    policyResource: "tool:company_document_update",
    description: "Update a registered company document's file path, title, counterpart, summary, or where it sits in the data room: its categoryCode, date, period, status, tags, hash, and the document it supersedes. Name the document with documentHint from a prior list or search result. Use when a file was moved or renamed so the ledger keeps tracking it, or when an administrator reclassifies it. Category changes require administrator access and move the document's files to the new category's folder.",
    version: "2",
    estimatedLatency: CapabilityEstimatedLatency.Low,
    inputSchema: companyDocumentUpdateInputSchema,
    inputIntentSchema: companyDocumentUpdateInputIntentSchema,
    result: { schema: companyDocumentResultSchema, effects: [{ objectType: "company_document", effect: ResourceMutationEffect.Updated, resultField: "documentID", effectIdentity: ResourceEffectIdentity.ID }] },
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
  },
  {
    name: "company_document_upload",
    namespace: "company",
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: "workspace_company",
    policyResource: "tool:company_document_upload",
    description: "Ask for a place in the company data room to put one file of a registered document. Answers the storagePath, which is the document's category folder and <name>.<documentID>.<extension>, and a signed URL to PUT the bytes to. Upload the original first with originalFileName, then each derived file with its fileName, which sits beside it as <name>.<documentID>.<fileName>. Whoever may change the document may upload its files. The URL is good for two hours.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Low,
    inputSchema: companyDocumentUploadInputSchema,
    inputIntentSchema: companyDocumentUploadInputIntentSchema,
    result: { schema: companyDocumentUploadResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
  },
  {
    name: "company_image_upload",
    namespace: "company",
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: "workspace_company",
    policyResource: "tool:company_image_upload",
    description: `Ask for a place to keep a new version of a company image, such as the seal stamped on forms or the letterhead logo. Each version is a data room document of its own, dated the day it took effect, and every form prints the newest. Answers the storagePath and a signed URL to PUT the image's bytes to, good for two hours. Once the image is there, keep it with company_info_set, passing the storagePath as ${serviceFileImageFieldNames}; keep a '${companyPictureImage}' with company_settings_update's profileImage instead, PUT with its image Content-Type. Only an administrator may keep a company image.`,
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Low,
    inputSchema: companyImageUploadInputSchema,
    inputIntentSchema: companyImageUploadInputIntentSchema,
    result: { schema: companyImageUploadResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
  },
  {
    name: "company_document_download",
    namespace: "company",
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: "workspace_company",
    policyResource: "tool:company_document_download",
    description: "Fetch a company document's original, or one of the files derived from it, out of the data room. Name the document with documentHint and add fileName for a derived file. Answers a signed URL good for ten minutes; category permissions and the share's download setting decide access. Read the sidecar and the summary first and fetch the original only when they cannot answer; an original goes to the requester themselves and passing it on is their own act.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Low,
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
    description: "Read the company master profile (name, representative, address, contact, bank account, country-specific legal attributes such as a business registration number). Without language it answers in the company's own language and names it as language; pass language ('ko' or 'en') for another document language. Either way missingFields lists empty core fields for that language. The answer also carries that view as the file company-profile.json, with the newest seal and logo images from the data room beside it, for a document to print from as it is. Call this before creating any company letterhead document; if missingFields is empty, never ask the user for company info again.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Low,
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
    description: `Save or update the company master profile. Partial update: only provided fields are written, into the given language's slot for localized fields. Use after the user supplies company details, or when they report one has changed. Put country-specific identifiers (a business registration number, a corporate registration number, an industry classification, an EIN …) into legalAttributes as a label-to-value JSON object string. A seal or logo image the user gives is kept with ${serviceFileImageFieldNames}, after company_image_upload.`,
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Low,
    inputSchema: companyInfoSetInputSchema,
    inputIntentSchema: companyInfoSetInputIntentSchema,
    result: { schema: companyProfileResultSchema, effects: [{ objectType: "company_info", effect: ResourceMutationEffect.Updated, effectIdentity: ResourceEffectIdentity.Singleton }] },
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
    description: "Record or correct one company metric value for a period — annual revenue, operating profit, MAU, employee count, GMV and similar time-series numbers. Money goes in amount with its USD equivalent; anything else goes in value with a unit. The period is a year, half, quarter, month or day. Same call overwrites the same period.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Low,
    inputSchema: companyMetricRecordInputSchema,
    inputIntentSchema: companyMetricRecordInputIntentSchema,
    result: { schema: companyMetricResultSchema, effects: [{ objectType: "company_metric", effect: ResourceMutationEffect.Updated, resultField: "metricID", effectIdentity: ResourceEffectIdentity.ID }] },
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
    inputSchema: companyRecordAddInputSchema,
    inputIntentSchema: companyRecordAddInputIntentSchema,
    result: { schema: companyRecordResultSchema, effects: [{ objectType: "company_record", effect: ResourceMutationEffect.Created, resultField: "recordID", effectIdentity: ResourceEffectIdentity.ID }] },
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
    inputSchema: companyRecordDeleteInputSchema,
    inputIntentSchema: companyRecordDeleteInputIntentSchema,
    result: { schema: companyRecordResultSchema, effects: [{ objectType: "company_record", effect: ResourceMutationEffect.Deleted, resultField: "recordID", effectIdentity: ResourceEffectIdentity.ID }] },
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
    inputSchema: companyRecordUpdateInputSchema,
    inputIntentSchema: companyRecordUpdateInputIntentSchema,
    result: { schema: companyRecordResultSchema, effects: [{ objectType: "company_record", effect: ResourceMutationEffect.Updated, resultField: "recordID", effectIdentity: ResourceEffectIdentity.ID }] },
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
  },
  {
    name: "company_equity_add",
    namespace: "company",
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: "workspace_company",
    policyResource: "tool:company_equity_add",
    description: "Add one line to the company's equity ledger: shares or options a holder received or gave up on a date, as the shareholder register or a share issuance records it. What anyone holds is the sum of their lines, so record a transfer as two lines, one negative and one positive. Only an administrator keeps the ledger.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Low,
    inputSchema: companyEquityAddInputSchema,
    inputIntentSchema: companyEquityAddInputIntentSchema,
    result: { schema: companyEquityEntryResultSchema, effects: [{ objectType: "company_equity_entry", effect: ResourceMutationEffect.Created, resultField: "entryID", effectIdentity: ResourceEffectIdentity.ID }] },
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
    completionEvidence: { mode: "success", action: "write_company", targetKind: "company" },
  },
  {
    name: "company_equity_list",
    namespace: "company",
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: "workspace_company",
    policyResource: "tool:company_equity_list",
    description: "List the company's equity ledger and what each holder holds as of a day, with their share of the total. Use for cap tables, ownership questions and dilution after a round.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Low,
    inputSchema: companyEquityListInputSchema,
    result: { schema: companyEquityListResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Read,
  },
  {
    name: "company_equity_delete",
    namespace: "company",
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: "workspace_company",
    policyResource: "tool:company_equity_delete",
    description: "Delete one wrong line of the company's equity ledger named by its entryID from company_equity_list. A real transfer or cancellation is a new line, not a delete.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Low,
    inputSchema: companyEquityDeleteInputSchema,
    inputIntentSchema: companyEquityDeleteInputIntentSchema,
    result: { schema: companyEquityEntryResultSchema, effects: [{ objectType: "company_equity_entry", effect: ResourceMutationEffect.Deleted, resultField: "entryID", effectIdentity: ResourceEffectIdentity.ID }] },
    sideEffect: CapabilitySideEffect.Destructive,
    requiresApproval: true,
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
export type CompanyDocumentListResult = z.infer<typeof companyDocumentListResultSchema>;
export type CompanyDocumentRecallResult = z.infer<typeof recallSourceResultSchema>;
export type CompanyDocumentPublished = z.infer<typeof companyDocumentPublishedSchema>;
export type CompanyDocumentUploadResult = z.infer<typeof companyDocumentUploadResultSchema>;
export type CompanyDocumentDownloadResult = z.infer<typeof companyDocumentDownloadResultSchema>;
export type CompanyImageUploadInput = z.infer<typeof companyImageUploadInputSchema>;
export type CompanyImageUploadResult = z.infer<typeof companyImageUploadResultSchema>;
