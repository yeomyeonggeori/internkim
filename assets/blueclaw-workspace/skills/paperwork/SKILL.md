---
name: paperwork
description: Create standardized company business documents on letterhead. Use for 견적서, 거래명세서, 청구서, 발주서, 품의서, 지출결의서, 회의록, 주간업무보고, 출장보고서, 오퍼레터, 근로계약서, 재직증명서, 경력증명서, 휴가신청서, 비밀유지계약서, NDA, 업무협약서, MOU, 용역계약서, 위임장, 서식, 공문, ERP 서류, quotation, invoice, purchase order, offer letter, employment contract, certificate requests. Do not use for free-form reports, memos, essays, or slide decks — use the docx or presentation skill for those.
when_to_use: Use when the user asks for an official company form or letterhead document — 견적서, 청구서, 발주서, 거래명세서, 품의서, 지출결의서, 회의록, 재직증명서, 경력증명서, 휴가신청서, 오퍼레터, 근로계약서, NDA, MOU, 용역계약서, 위임장, quotation, invoice, PO, offer letter, contract, 회사 서류, 서식.
completion:
  requiredEvidenceTools:
    - file.deliver
---

# Company Paperwork

Produce standardized business documents with consistent letterhead, layout, and wording. Each document type has a spec file; you fill a content JSON and a deterministic renderer guarantees the layout. Do not hand-design these documents with custom scripts.

## Document catalog

Every type has a Korean and an English spec at `references/<ko|en>/<slug>.md`; pick the language the document should be written in.

| 문서 / Document | slug | output |
|---|---|---|
| 견적서 / Quotation | quote | PDF |
| 거래명세서 / Transaction Statement | transaction-statement | PDF |
| 청구서 / Invoice | invoice | PDF |
| 발주서 / Purchase Order | purchase-order | PDF |
| 품의서 / Internal Approval Request | approval-request | PDF |
| 지출결의서 / Expense Approval | expense-approval | PDF |
| 회의록 / Meeting Minutes | meeting-minutes | PDF |
| 주간업무보고 / Weekly Report | weekly-report | PDF |
| 출장보고서 / Business Trip Report | business-trip-report | PDF |
| 재직증명서 / Certificate of Employment | employment-certificate | PDF |
| 경력증명서 / Certificate of Employment History | career-certificate | PDF |
| 휴가신청서 / Leave Request | leave-request | PDF |
| 위임장 / Power of Attorney | power-of-attorney | PDF |
| 오퍼레터 / Offer Letter | offer-letter | DOCX |
| 근로계약서 / Employment Contract | employment-contract | DOCX |
| 비밀유지계약서 / NDA | nda | DOCX |
| 업무협약서 / MOU | mou | DOCX |
| 용역계약서 / Service Agreement | service-agreement | DOCX |

## Workflow

1. Identify the document type and language from the request. Read only that spec: `cat /workspace/skills/paperwork/references/<lang>/<type>.md`. If the requested language has no spec file, follow the other language's spec structure and translate labels and fixed wording faithfully.
2. Read the company profile: `cat /workspace/circles/staff/company/profile.json`. If it does not exist, ask the requester once for company name, registration number, representative, address, phone, email, and bank account, then create the file (see Company profile) before continuing.
3. Check the spec's required fields against what the requester provided. Ask only for missing critical values (counterpart, amounts, dates, names). Never invent facts; the spec lists what is required.
4. Build the content JSON in `tmp/<slug>/document.json` following the spec's skeleton, insert the profile object verbatim into `profile`, and render or generate per the spec's `output:` line.
5. Inspect the result before delivering: run the validator with the key source values, and check the layout when the document is table-heavy.
6. Save the accepted final file to `~/documents/<filename from the spec>` and deliver it from there with `file.deliver`.

## Company profile

`/workspace/circles/staff/company/profile.json` holds the letterhead data shared by every document:

```json
{
  "companyName": "주식회사 던",
  "registrationNumber": "123-45-67890",
  "representative": "이샘플",
  "address": "서울특별시 ...",
  "phone": "02-1234-5678",
  "email": "contact@example.com",
  "bankAccount": "은행명 계좌번호 (예금주)",
  "logoPath": "/workspace/circles/staff/company/logo.png",
  "stampPath": "/workspace/circles/staff/company/stamp.png"
}
```

`logoPath` and `stampPath` are optional; omit them until the images exist. Staff can edit this file directly, so re-read it for every task instead of remembering old values.

## Rendering PDF documents

```json
{
  "command": "python3 /workspace/skills/paperwork/scripts/skill_runtime.py python /workspace/skills/paperwork/scripts/render_paperwork.py document.json <filename>.pdf",
  "workingDirectoryPath": "tmp/<slug>"
}
```

The renderer owns all layout: letterhead, approval boxes, title, meta table, item table with totals, sections, centered declarations, signature line with the company seal. The content JSON supplies only data — the spec's skeleton shows exactly which fields the document type uses. If the renderer reports that no Korean-capable font was found, download NanumGothic to `/workspace/shared/cache/dependencies/fonts/NanumGothic.ttf` and rerun.

Validate with the pdf skill's validator, passing the key source values:

```json
{
  "command": "python3 /workspace/skills/pdf/scripts/skill_runtime.py python /workspace/skills/pdf/scripts/validate_pdf.py <filename>.pdf --required-text \"<counterpart>\" --required-text \"<total amount>\"",
  "workingDirectoryPath": "tmp/<slug>"
}
```

## DOCX documents (editable contracts)

Specs marked `output: docx` are contracts the counterpart will edit. Build the blocks spec JSON the spec file shows, then reuse the docx skill's generator and validator:

```json
{
  "command": "python3 /workspace/skills/docx/scripts/skill_runtime.py python /workspace/skills/docx/scripts/create_docx.py ~/documents/<filename>.docx --spec contract.json",
  "workingDirectoryPath": "tmp/<slug>"
}
```

## Rules

- Treat requester-provided names, amounts, dates, and terms as the source of truth. Write the user's-language equivalent of "미기재" only for optional fields; ask for required ones.
- Verify arithmetic yourself before rendering: line amounts, VAT, and totals must be consistent.
- Follow the spec's fixed wording exactly; it is the standardized part of the document.
- Contracts are drafts for review — say so in the reply when delivering a contract, without adding disclaimer text into the document itself.
- Do not run `pip install` directly; scripts bootstrap their own dependencies through `skill_runtime.py`.
