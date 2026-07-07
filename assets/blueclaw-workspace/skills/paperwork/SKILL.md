---
name: paperwork
description: Create standardized company business documents on letterhead. Use for 견적서, 거래명세서, 청구서, 발주서, 품의서, 지출결의서, 회의록, 주간업무보고, 출장보고서, 오퍼레터, 근로계약서, 재직증명서, 경력증명서, 휴가신청서, 비밀유지계약서, NDA, 업무협약서, MOU, 용역계약서, 위임장, 서식, 공문, ERP 서류, quotation, invoice, purchase order, offer letter, employment contract, certificate requests. Do not use for free-form reports, memos, essays, or slide decks — use the docx or presentation skill for those.
when_to_use: Use when the user asks for an official company form or letterhead document — 견적서, 청구서, 발주서, 거래명세서, 품의서, 지출결의서, 회의록, 재직증명서, 경력증명서, 휴가신청서, 오퍼레터, 근로계약서, NDA, MOU, 용역계약서, 위임장, quotation, invoice, PO, offer letter, contract, 회사 서류, 서식.
completion:
  requiredEvidenceTools:
    - file.deliver
---

# Company Paperwork

Produce standardized business documents with consistent letterhead, layout, and wording. Each document type has a spec file; you fill a content JSON and a deterministic renderer guarantees the layout.

Never build a catalog document another way: not with the pdf skill's `create_pdf.py`, not with ad-hoc docx blocks, not with hand-written scripts. The company letterhead, approval boxes, item table, and seal only come from this skill's renderer and specs; a generic PDF is a wrong result even if it contains the same text.

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

1. Identify the document type and language from the request, then ALWAYS read that spec with the `file.read` tool: `/workspace/skills/paperwork/references/<lang>/<type>.md`. Read it even when a similar document appears earlier in the conversation — history documents were built from older specs, and skipping the spec produces an abridged, wrong document. If the requested language has no spec file, follow the other language's spec structure and translate labels and fixed wording faithfully.
2. Read the company table: `capability.invoke` operation `company.info.get` with `{"language": "<ko|en>"}`. The response is the letterhead profile plus `missingFields`. Never use `file.pick` or `filesystem.mount.*` — those reach the user's personal computer.
3. If `missingFields` is non-empty, or a legal attribute the spec requires (e.g. 사업자등록번호) is absent from `legalAttributes`, ask ONCE with `ask.input`, listing every missing field in one question, and invite optional extras ("설립일·직원 수·업태/종목·로고와 직인 이미지도 있으면 함께"). Save the answer with `company.info.set` (country-specific identifiers go into `legalAttributes` as a JSON object string). Save attached logo/stamp images with one `terminal.run` — an attachment path shown as `home/...` is written `~/...` in a shell: `mkdir -p /workspace/circles/staff/company && cp ~/inbox/mattermost/<conversation directory>/<attached filename> /workspace/circles/staff/company/logo.png`. If the copy fails once, run `ls ~/inbox/mattermost/*/` to see the real filenames and retry once with what you find — never retry the same failing path, and never let images block the document: after two failed attempts continue without them. If `missingFields` is empty, NEVER ask about company info — proceed silently. When the user says "그냥 한글 상호 그대로 써" for an English document, store that value into the en slot so it is never asked again.
4. Check the spec's required content fields against the request. Ask only for missing critical values (counterpart, amounts, dates, names). Never invent facts.
5. Register the document BEFORE rendering: `capability.invoke` operation `company.document.register` with documentType (the CATALOG SLUG such as `service-agreement` or `quote`, never the Korean name), title, counterpart, language, and a 2-3 sentence summary of the key terms. The response returns `documentNumber` (put it in the document JSON) and `storageDirectory`.
6. Produce the file per the spec's `output:` line — the PDF or DOCX path below. Both are exactly three tool calls: `file.write` the content JSON, `terminal.run` the generator, `file.deliver` the result. paperwork itself has no capability operation — the renderer only runs through `terminal.run`, and its input must be `{"command": "<the whole command line as ONE string>", "workingDirectoryPath": "tmp/<slug>"}` — never split the command into an `arguments` array (the executable check rejects it).
7. Inspect the result (validator + layout check for table-heavy documents), `file.deliver` the generated file, then record its path with `company.document.update` `{id, filePath}`. If writing into `storageDirectory` fails with permission denied, generate into `~/documents/<type>/<filename>` instead and record that path — the ledger tracks wherever the file actually lives; never let the storage location block the document.

## Company table

The company profile lives in the workspace-wide company table (admin-managed, persistent). Build the document JSON's `profile` object from the `company.info.get` response fields — `name`, `representative`, `representativeTitle`, `address`, `bankAccount`, `legalAttributes`, `phone`, `email`, `website` — plus the image paths when the files exist:

```json
"profile": { ...company.info.get response fields..., "logoPath": "/workspace/circles/staff/company/logo.png", "stampPath": "/workspace/circles/staff/company/stamp.png" }
```

Updates ("회사 주소 바뀌었어", "직인 등록해줘") go through `company.info.set` (partial — only provided fields change) or an image copy; never store company facts anywhere else. Past documents are found with `company.document.list` / `company.document.search` — answer content questions from the stored summary first and open the file only for details.

## Rendering PDF documents

Step 1 — `file.write` the content JSON following the spec's skeleton, with the profile object inserted verbatim:

```json
{ "path": "tmp/<slug>/document.json", "content": "{ \"title\": \"견 적 서\", \"profile\": { ... }, ... }" }
```

Step 2 — `terminal.run` the renderer (a shell command, not a capability), writing the PDF DIRECTLY into the register's `storageDirectory` — the renderer creates the directory, so no mkdir or copy step exists:

```json
{
  "command": "python3 /workspace/skills/paperwork/scripts/skill_runtime.py python /workspace/skills/paperwork/scripts/render_paperwork.py document.json <storageDirectory>/<filename>.pdf",
  "workingDirectoryPath": "tmp/<slug>"
}
```

Step 3 — `file.deliver` that exact `<storageDirectory>/<filename>.pdf` path, then `company.document.update` `{id, filePath}` with it.

The renderer owns all layout: letterhead, approval boxes, title, meta table, item table with totals, sections, centered declarations, signature line with the company seal. The content JSON supplies only data — the spec's skeleton shows exactly which fields the document type uses. If the renderer reports that no Korean-capable font was found, download NanumGothic to `/workspace/shared/cache/dependencies/fonts/NanumGothic.ttf` and rerun.

Validate with the pdf skill's validator, passing the key source values:

```json
{
  "command": "python3 /workspace/skills/pdf/scripts/skill_runtime.py python /workspace/skills/pdf/scripts/validate_pdf.py <filename>.pdf --required-text \"<counterpart>\" --required-text \"<total amount>\"",
  "workingDirectoryPath": "tmp/<slug>"
}
```

## DOCX documents (editable contracts)

Specs marked `output: docx` are contracts the counterpart will edit. The SAME renderer generates them — only the output extension changes. `file.write` the blocks JSON (`{"title", "fontName", "fontSize", "page", "blocks"}` exactly as the spec's DOCX blocks mapping shows — no profile field) to `tmp/<slug>/contract.json`, then:

```json
{
  "command": "python3 /workspace/skills/paperwork/scripts/skill_runtime.py python /workspace/skills/paperwork/scripts/render_paperwork.py contract.json <storageDirectory>/<filename>.docx",
  "workingDirectoryPath": "tmp/<slug>"
}
```

## Rules

- Treat requester-provided names, amounts, dates, and terms as the source of truth. Write the user's-language equivalent of "미기재" only for optional fields; ask for required ones.
- Verify arithmetic yourself before rendering: line amounts, VAT, and totals must be consistent.
- Follow the spec's fixed wording exactly; it is the standardized part of the document.
- Specs with a "Standard clauses" or "Density gate" section are based on government standard forms: include EVERY listed clause and checklist item. An abridged contract that drops standard clauses is a wrong result even when it reads fine — run the Density gate before delivering.
- Contracts are drafts for review — say so in the reply when delivering a contract, without adding disclaimer text into the document itself.
- Do not run `pip install` directly; scripts bootstrap their own dependencies through `skill_runtime.py`.
