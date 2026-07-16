---
name: company-data
description: Record and look up company master data — metrics time series (연매출, 영업이익, MAU, 직원 수), history and assets (연혁, 투자 유치, 제품 출시, 특허, 인증, 수상, 레퍼런스), and the company document ledger. Use for 매출 기록, 지표 기록, 연혁 추가, 투자 이력, 회사 정보 수정, 우리가 보낸 계약서/견적서 조회, revenue record, funding history, company timeline requests. Do not use for creating documents — the paperwork skill owns document generation.
allowed-tools: file.read company.info.get company.info.set company.metric.list company.metric.record company.record.list company.record.add company.record.update company.record.delete company.document.list company.document.search company.document.register
---

# Company Data

The company has one persistent master table set: profile (`company.info.get`, `company.info.set`), numeric time series (`company.metric.list`, `company.metric.record`), history/asset records (`company.record.list`, `company.record.add`, `company.record.update`, `company.record.delete`), and the document ledger (`company.document.list`, `company.document.search`, `company.document.register`). Call these typed tools directly. Record facts the moment the user states them; answer questions from these tables instead of guessing.

## Metrics — 수치 시계열

"작년 연매출은 12억이었어" → FIRST check what already exists for that year, THEN write:

1. Call `company.metric.list` with `{"fromYear": 2025, "toYear": 2025}` — all rows for the year (add `metric` to narrow).
2. An existing row already states the identical fact? No write — just confirm it is on record.
3. An existing row states the same fact with different value or granularity (quarter/month)? Re-record THAT row's period with the corrected value — do not add a second row for the same fact.
4. Nothing overlaps? Add the new row:

```json
{ "metric": "annualRevenue", "year": 2025, "value": 1200000000, "currency": "KRW", "valueUSD": 870000 }
```

- Period granularity: year only = annual, add `quarter` (1-4) OR `month` (1-12) — never both.
- Reuse the same metric key across periods: `annualRevenue`, `operatingProfit`, `mau`, `employees`, `gmv`.
- Money uses `currency`: `USD`, `KRW`, `EUR`, `JPY`, `GBP`, `CNY`, `HKD`, `SGD`, `AUD`, `CAD`, `CHF`, or `INR`. `value` is the stated local amount; non-USD money also requires the stated USD equivalent in `valueUSD`. Never estimate an exchange rate.
- Non-monetary metrics use `unit` instead of `currency`. Do not combine them.
- `company.metric.record` upserts on (metric, year, quarter, month) — a correction is the same call with the same period.

## Records — 연혁·투자·제품·인증

"작년 11월에 시드로 20억 투자받았어" → FIRST call `company.record.list` with `{"category": "funding"}` and scan for the same event in that year. Already recorded identically? No write — confirm it exists. Same event with different details? Call `company.record.update` with the id from the list. Nothing overlaps? Call `company.record.add`:

```json
{ "category": "funding", "date": "2025-11", "title": "시드 투자 유치", "attributes": "{\"round\": \"Seed\", \"amount\": \"20억 원\", \"investors\": \"ABC벤처스\"}" }
```

- Categories: `history`(연혁), `funding`, `product`, `certification`, `ip`(특허), `award`, `reference`(주요 고객·파트너), `grant`(정부과제). Pick the closest; new kebab-case categories are allowed.
- `company.record.delete` only on explicit user request.

## Profile — 회사 기본 정보

"회사 주소 바뀌었어" → call `company.info.set` with `language` plus only the changed fields. Read with `company.info.get` and `{"language": "ko"}`. Country-specific identifiers (사업자등록번호, 업태, 종목, EIN …) go into `legalAttributes` as a JSON object string. Never store company facts in files or memory instead of this table.

## Document ledger — 발급·수신 문서

"ABC랑 맺은 계약 조건이 뭐였지" → call `company.document.search` with `{"query": "ABC 용역 계약 대금"}`. Answer from the returned summaries; call `file.read` only when the summary is not enough. "ABC에 보낸 견적서 목록" → call `company.document.list`. A received contract the user attaches for safekeeping → save the file under `/workspace/circles/staff/documents/<type>/`, then call `company.document.register` with `kind: "received"` and a 2-3 sentence summary.

## Table schema

- `company_metrics` — key `(metric, year, quarter, month)`, columns `value`, `currency`, `valueUSD`, `unit`, `note`, `updatedAt`. Money uses `currency` and a stable `valueUSD`; other metrics use `unit`. quarter/month are 0 when unset; quarter AND month together is rejected.
- `company_records` — `id`, `category`, `date` (YYYY-MM[-DD]), `title`, `detail`, `attributes` (free JSON object per category), `updatedAt`.
- `company_documents` — `id`, `documentNumber` (server-assigned, issued only), `kind` (issued/received/internal), `documentType`, `title`, `counterpart`, `language`, `filePath`, `summary`, `updatedAt`.
- Profile (`company.info.get`, `company.info.set`) — localized: `name`, `brandName`, `slogan`, `description`, `representative`, `representativeTitle`, `address`, `officeAddress`, `jurisdiction`, `bankAccount`, `legalAttributes` (label→value map); neutral: `foundedDate`, `capital`, `fiscalYearEnd`, `employeeCount`, `phone`, `fax`, `email`, `website`.

## Rules

- Never blindly append: search existing rows for the year/category first; identical → no write, overlap → update, add only genuinely new facts.
- Record only user-stated facts — never estimate values, dates, or investor names.
- Confirm the stored result back to the user in one line (metric, period, value) so mistakes surface immediately.
- These tables feed IR decks, business plans, and grant applications later — prefer structured `attributes` over prose in `detail`.
