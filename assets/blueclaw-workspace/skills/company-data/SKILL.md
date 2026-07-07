---
name: company-data
description: Record and look up company master data — metrics time series (연매출, 영업이익, MAU, 직원 수), history and assets (연혁, 투자 유치, 제품 출시, 특허, 인증, 수상, 레퍼런스), and the company document ledger. Use for 매출 기록, 지표 기록, 연혁 추가, 투자 이력, 회사 정보 수정, 우리가 보낸 계약서/견적서 조회, revenue record, funding history, company timeline requests. Do not use for creating documents — the paperwork skill owns document generation.
when_to_use: Use when the user records or asks about company facts — "작년 매출 12억 기록해둬", "시드 투자 받은 거 등록", "우리 연혁 보여줘", "직원 수 업데이트", "ABC랑 맺은 계약 내용이 뭐였지", "회사 주소 바뀌었어", record revenue, funding round, company history, past documents.
---

# Company Data

The company has one persistent master table set: profile (`company.info.*`), numeric time series (`company.metric.*`), history/asset records (`company.record.*`), and the document ledger (`company.document.*`). All are `capability.invoke` operations. Record facts the moment the user states them; answer questions from these tables instead of guessing.

## Metrics — 수치 시계열

"작년 연매출은 12억이었어" → `company.metric.record`:

```json
{ "operation": "company.metric.record", "input": "{\"metric\": \"annualRevenue\", \"year\": 2025, \"value\": 1200000000, \"unit\": \"KRW\"}" }
```

- Period granularity: year only = annual, add `quarter` (1-4) OR `month` (1-12) — never both.
- Reuse the same metric key across periods: `annualRevenue`, `operatingProfit`, `mau`, `employees`, `gmv`.
- Corrections are the same call with the same period. Read with `company.metric.list` (`metric`, `fromYear`, `toYear`).

## Records — 연혁·투자·제품·인증

"작년 11월에 시드로 20억 투자받았어" → `company.record.add`:

```json
{ "operation": "company.record.add", "input": "{\"category\": \"funding\", \"date\": \"2025-11\", \"title\": \"시드 투자 유치\", \"attributes\": \"{\\\"round\\\": \\\"Seed\\\", \\\"amount\\\": \\\"20억 원\\\", \\\"investors\\\": \\\"ABC벤처스\\\"}\"}" }
```

- Categories: `history`(연혁), `funding`, `product`, `certification`, `ip`(특허), `award`, `reference`(주요 고객·파트너), `grant`(정부과제). Pick the closest; new kebab-case categories are allowed.
- Query with `company.record.list` (`category`, `query`); fix with `company.record.update` (id from list); `company.record.delete` only on explicit user request.

## Profile — 회사 기본 정보

"회사 주소 바뀌었어" → `company.info.set` with `language` plus only the changed fields. Read with `company.info.get {"language": "ko"}`. Country-specific identifiers (사업자등록번호, 업태, 종목, EIN …) go into `legalAttributes` as a JSON object string. Never store company facts in files or memory instead of this table.

## Document ledger — 발급·수신 문서

"ABC랑 맺은 계약 조건이 뭐였지" → `company.document.search {"query": "ABC 용역 계약 대금"}`. Answer from the returned summaries; open the file with `file.read` only when the summary is not enough. "ABC에 보낸 견적서 목록" → `company.document.list`. A received contract the user attaches for safekeeping → save the file under `/workspace/circles/staff/documents/<type>/`, then `company.document.register` with `kind: "received"` and a 2-3 sentence summary.

## Rules

- Record only user-stated facts — never estimate values, dates, or investor names.
- Confirm the stored result back to the user in one line (metric, period, value) so mistakes surface immediately.
- These tables feed IR decks, business plans, and grant applications later — prefer structured `attributes` over prose in `detail`.
