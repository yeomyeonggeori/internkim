import { z } from 'zod';

import {
  CapabilityAnsweredBy,
  CapabilityEstimatedLatency,
  CapabilityModelVisibility,
  CapabilitySideEffect,
} from './protocol';

import type { CapabilityToolDefinition } from './definition';

const companyDocumentListInputSchema = z.strictObject({
  counterpart: z.string().describe("Counterpart name to filter by, e.g. 'ABC물산'.").optional(),
  query: z.string().describe("Keyword filter matched against title, summary, and counterpart.").optional(),
  type: z.string().describe("Document type slug to filter by, e.g. 'quote'. Leave empty for all types.").optional(),
});

const companyDocumentRegisterInputSchema = z.strictObject({
  counterpart: z.string().describe("Counterpart company or person name, e.g. 'ABC물산'.").optional(),
  documentType: z.string().describe("Document type slug from the paperwork catalog, e.g. 'quote', 'service-agreement', 'employment-certificate'."),
  filePath: z.string().describe("Workspace path of the file if it already exists. For issued documents you can also set it later with company_document_update after saving.").optional(),
  kind: z.string().describe("'issued' for documents the company creates (default, gets a document number), 'received' for documents from counterparts, 'internal' for internal-only files.").optional(),
  language: z.string().describe("Document language, e.g. 'ko' or 'en'.").optional(),
  summary: z.string().describe("2-3 sentence summary of the document's key terms: parties, amounts, dates, obligations. Written so later questions can be answered without opening the file."),
  title: z.string().describe("Document title including the counterpart, e.g. 'ABC물산 김인턴 도입 컨설팅 견적서'."),
});

const companyDocumentSearchInputSchema = z.strictObject({
  limit: z.int().describe("Maximum documents to return. Defaults to 5.").optional(),
  query: z.string().describe("Natural-language question or topic, e.g. 'ABC물산이랑 맺은 용역 계약 대금 조건'."),
});

const companyDocumentUpdateInputSchema = z.strictObject({
  counterpart: z.string().describe("Corrected counterpart. Omit to keep unchanged.").optional(),
  filePath: z.string().describe("New workspace path after the file was saved, moved, or renamed.").optional(),
  id: z.string().describe("Document id from a prior company_document_list or search result."),
  summary: z.string().describe("Replacement summary. Omit to keep unchanged.").optional(),
  title: z.string().describe("Corrected title. Omit to keep unchanged.").optional(),
});

const companyInfoGetInputSchema = z.strictObject({
  language: z.string().describe("Document language to resolve the profile for, e.g. 'ko' or 'en'. Defaults to 'ko'. The response's missingFields lists core fields still empty for this language.").optional(),
});

const companyInfoSetInputSchema = z.strictObject({
  address: z.string().describe("Registered head-office address.").optional(),
  bankAccount: z.string().describe("One-line bank account: bank, account number, holder. Use the 'en' slot for international wire details (SWIFT/IBAN).").optional(),
  brandName: z.string().describe("Service or brand name when it differs from the legal name.").optional(),
  capital: z.string().describe("Paid-in capital, e.g. '5억 원'.").optional(),
  description: z.string().describe("One-line company description for proposals and IR material.").optional(),
  email: z.string().describe("Main company email address.").optional(),
  employeeCount: z.int().describe("Official employee headcount. Independent of platform member count.").optional(),
  fax: z.string().describe("Fax number.").optional(),
  fiscalYearEnd: z.string().describe("Fiscal year end month, e.g. '12월'.").optional(),
  foundedDate: z.string().describe("Founding date in YYYY-MM-DD format.").optional(),
  jurisdiction: z.string().describe("Jurisdiction of incorporation for contract preambles, e.g. '대한민국' or 'the State of Delaware'.").optional(),
  language: z.string().describe("Language slot the localized values belong to, e.g. 'ko' or 'en'."),
  legalAttributes: z.string().describe("JSON object string of country-specific label-to-value pairs, e.g. {\"사업자등록번호\": \"123-45-67890\", \"업태\": \"서비스\"}. Labels print on documents as-is.").optional(),
  name: z.string().describe("Legal company name, e.g. '주식회사 여명거리'.").optional(),
  officeAddress: z.string().describe("Working office address when it differs from the registered address.").optional(),
  phone: z.string().describe("Main company phone number.").optional(),
  representative: z.string().describe("Representative's name; use the romanized name for the 'en' slot.").optional(),
  representativeTitle: z.string().describe("Representative's title. Defaults to 대표이사 (ko) / CEO (en).").optional(),
  slogan: z.string().describe("Company slogan for letterheads and introductions.").optional(),
  website: z.string().describe("Company website URL.").optional(),
});

const companyMetricListInputSchema = z.strictObject({
  fromYear: z.int().describe("Earliest year to include.").optional(),
  metric: z.string().describe("Metric key to filter by, e.g. 'annualRevenue'. Leave empty for all metrics.").optional(),
  toYear: z.int().describe("Latest year to include.").optional(),
});

const companyMetricRecordInputSchema = z.strictObject({
  currency: z.enum(["USD", "KRW", "EUR", "JPY", "GBP", "CNY", "HKD", "SGD", "AUD", "CAD", "CHF", "INR"]).describe("ISO currency of a monetary value. Use currency instead of unit for money.").optional(),
  metric: z.string().describe("Metric key in lowerCamelCase, e.g. 'annualRevenue', 'operatingProfit', 'mau', 'employees'. Reuse the same key across periods."),
  month: z.int().describe("Month 1-12 for a monthly value. Leave out for annual or quarterly values; never combine with quarter.").optional(),
  note: z.string().describe("Source or context note, e.g. '재무제표 기준'.").optional(),
  quarter: z.int().describe("Quarter 1-4 for a quarterly value. Leave out for annual or monthly values; never combine with month.").optional(),
  unit: z.string().describe("Unit for a non-monetary value, e.g. '명', '건', 'sites'. Do not combine with currency.").optional(),
  value: z.number().describe("Numeric value, e.g. 1200000000 for 12억 원. Use the raw number, not a formatted string."),
  valueUSD: z.number().describe("Stable USD equivalent for a non-USD monetary value. Required when currency is not USD; omitted for non-monetary values.").optional(),
  year: z.int().describe("Four-digit year the value belongs to, e.g. 2025."),
});

const companyRecordAddInputSchema = z.strictObject({
  attributes: z.string().describe("JSON object string of structured details, e.g. {\"round\": \"Seed\", \"amount\": \"20억 원\", \"investors\": \"ABC벤처스\"}.").optional(),
  category: z.string().describe("Record category: 'history' (연혁), 'funding', 'product', 'certification', 'ip', 'award', 'reference', 'grant', or another short kebab-case label."),
  date: z.string().describe("Date of the event in YYYY-MM-DD or YYYY-MM format. Used to sort the company timeline.").optional(),
  detail: z.string().describe("One-to-three sentence description.").optional(),
  title: z.string().describe("Short title, e.g. '시드 투자 유치' or '김인턴 정식 출시'."),
});

const companyRecordDeleteInputSchema = z.strictObject({
  id: z.string().describe("Record id from a prior company_record_list result. Never invent an id."),
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
  id: z.string().describe("Record id from a prior company_record_list result."),
  title: z.string().describe("New title. Omit to keep unchanged.").optional(),
});

export const companyToolDefinitions: CapabilityToolDefinition[] = [
  {
    name: "company_document_list",
    namespace: "company",
    answeredBy: CapabilityAnsweredBy.Company,
    privacyClass: "workspace_company",
    policyResource: "tool:company_document_list",
    description: "List registered company documents newest first, with their numbers, counterparts, file paths, and summaries. Filter by type, counterpart, or keyword. Use to answer 'what quotes did we send to X'.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Low,
    modelVisibility: CapabilityModelVisibility.Hidden,
    inputSchema: companyDocumentListInputSchema,
    sideEffect: CapabilitySideEffect.Read,
    worksOffline: true,
  },
  {
    name: "company_document_register",
    namespace: "company",
    answeredBy: CapabilityAnsweredBy.Company,
    privacyClass: "workspace_company",
    policyResource: "tool:company_document_register",
    description: "Register a company document in the document ledger and, for kind=issued, receive the official document number to print in the document plus the storage directory to save the final file in. Call BEFORE rendering an official document so the number appears in it. Always include a 2-3 sentence summary of the document's key terms (parties, amounts, dates) so later questions can be answered without re-reading the file.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Low,
    modelVisibility: CapabilityModelVisibility.Hidden,
    inputSchema: companyDocumentRegisterInputSchema,
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
    worksOffline: true,
    completionEvidence: { mode: "success", action: "write_company", targetKind: "company" },
  },
  {
    name: "company_document_search",
    namespace: "company",
    answeredBy: CapabilityAnsweredBy.Company,
    privacyClass: "workspace_company",
    policyResource: "tool:company_document_search",
    description: "Semantically search registered company documents by a natural-language question ('ABC와 맺은 계약 조건'). Returns best-matching documents with summaries — answer from the summary first and open the file only when detail is needed.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    modelVisibility: CapabilityModelVisibility.Hidden,
    inputSchema: companyDocumentSearchInputSchema,
    sideEffect: CapabilitySideEffect.Read,
    worksOffline: true,
  },
  {
    name: "company_document_update",
    namespace: "company",
    answeredBy: CapabilityAnsweredBy.Company,
    privacyClass: "workspace_company",
    policyResource: "tool:company_document_update",
    description: "Update a registered document's file path, title, counterpart, or summary by id from a prior list/search result. Use when a file was moved or renamed so the ledger keeps tracking it.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Low,
    modelVisibility: CapabilityModelVisibility.Hidden,
    inputSchema: companyDocumentUpdateInputSchema,
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
    worksOffline: true,
  },
  {
    name: "company_info_get",
    namespace: "company",
    answeredBy: CapabilityAnsweredBy.Company,
    privacyClass: "workspace_company",
    policyResource: "tool:company_info_get",
    description: "Read the company master profile (name, representative, address, contact, bank account, country-specific legal attributes such as 사업자등록번호). Pass language ('ko' or 'en') to get the view for that document language plus missingFields listing empty core fields. Call this before creating any company letterhead document; if missingFields is empty, never ask the user for company info again.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Low,
    modelVisibility: CapabilityModelVisibility.Hidden,
    inputSchema: companyInfoGetInputSchema,
    sideEffect: CapabilitySideEffect.Read,
    worksOffline: true,
  },
  {
    name: "company_info_set",
    namespace: "company",
    answeredBy: CapabilityAnsweredBy.Company,
    privacyClass: "workspace_company",
    policyResource: "tool:company_info_set",
    description: "Save or update the company master profile. Partial update: only provided fields are written, into the given language's slot for localized fields. Use after the user supplies company details, or when they report a change ('회사 주소 바뀌었어'). Put country-specific identifiers (사업자등록번호, 법인등록번호, 업태, 종목, EIN …) into legalAttributes as a label-to-value JSON object string.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Low,
    modelVisibility: CapabilityModelVisibility.Hidden,
    inputSchema: companyInfoSetInputSchema,
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
    worksOffline: true,
    completionEvidence: { mode: "success", action: "write_company", targetKind: "company" },
  },
  {
    name: "company_metric_list",
    namespace: "company",
    answeredBy: CapabilityAnsweredBy.Company,
    privacyClass: "workspace_company",
    policyResource: "tool:company_metric_list",
    description: "List recorded company metrics sorted by period. Filter by metric key and year range. Use for IR decks, business plans, and grant applications that need revenue/headcount/usage time series.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Low,
    modelVisibility: CapabilityModelVisibility.Hidden,
    inputSchema: companyMetricListInputSchema,
    sideEffect: CapabilitySideEffect.Read,
    worksOffline: true,
  },
  {
    name: "company_metric_record",
    namespace: "company",
    answeredBy: CapabilityAnsweredBy.Company,
    privacyClass: "workspace_company",
    policyResource: "tool:company_metric_record",
    description: "Record or correct one company metric value for a period — annual revenue, operating profit, MAU, employee count, GMV and similar time-series numbers. Monetary values use an allowed currency plus a stable USD equivalent; non-monetary values use unit. Add quarter (1-4) OR month (1-12) for sub-annual periods, never both. Same call overwrites the same period.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Low,
    modelVisibility: CapabilityModelVisibility.Hidden,
    inputSchema: companyMetricRecordInputSchema,
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
    worksOffline: true,
    completionEvidence: { mode: "success", action: "write_company", targetKind: "company" },
  },
  {
    name: "company_record_add",
    namespace: "company",
    answeredBy: CapabilityAnsweredBy.Company,
    privacyClass: "workspace_company",
    policyResource: "tool:company_record_add",
    description: "Add one company history/asset record: milestones (연혁), funding rounds, products, patents, certifications, awards, client references, government grants. Set category, date, title; put structured details (amount, investors, round …) into attributes as a label-to-value JSON object string.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Low,
    modelVisibility: CapabilityModelVisibility.Hidden,
    inputSchema: companyRecordAddInputSchema,
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
    worksOffline: true,
    completionEvidence: { mode: "success", action: "write_company", targetKind: "company" },
  },
  {
    name: "company_record_delete",
    namespace: "company",
    answeredBy: CapabilityAnsweredBy.Company,
    privacyClass: "workspace_company",
    policyResource: "tool:company_record_delete",
    description: "Delete a company record by id from a prior company_record_list result. Use only when the user asks to remove a wrong entry.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Low,
    modelVisibility: CapabilityModelVisibility.Hidden,
    inputSchema: companyRecordDeleteInputSchema,
    sideEffect: CapabilitySideEffect.Destructive,
    requiresApproval: true,
    worksOffline: true,
  },
  {
    name: "company_record_list",
    namespace: "company",
    answeredBy: CapabilityAnsweredBy.Company,
    privacyClass: "workspace_company",
    policyResource: "tool:company_record_list",
    description: "List company history/asset records, newest first. Filter by category (history, funding, product, certification, ip, award, reference, grant …) or keyword query. Use to build 연혁 sections, funding tables, and product overviews.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Low,
    modelVisibility: CapabilityModelVisibility.Hidden,
    inputSchema: companyRecordListInputSchema,
    sideEffect: CapabilitySideEffect.Read,
    worksOffline: true,
  },
  {
    name: "company_record_update",
    namespace: "company",
    answeredBy: CapabilityAnsweredBy.Company,
    privacyClass: "workspace_company",
    policyResource: "tool:company_record_update",
    description: "Update fields on an existing company record identified by id from a prior company_record_list result. Only provided fields change.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Low,
    modelVisibility: CapabilityModelVisibility.Hidden,
    inputSchema: companyRecordUpdateInputSchema,
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
    worksOffline: true,
  },
];
