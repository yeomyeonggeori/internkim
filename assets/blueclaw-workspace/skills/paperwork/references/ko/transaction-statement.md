# 거래명세서 (Transaction Statement)

output: pdf
filename: 거래명세서_<공급받는자>_<YYYYMMDD>.pdf

## Purpose

완료된 거래 내역을 공급자와 공급받는자 사이에 기록으로 남기는 대외 문서. 견적서와 달리 확정 거래의 실제 품목·수량·금액을 옮겨 적는다.

## Required fields

- recipient: 공급받는자 회사명 필수, 담당자명은 있으면 함께
- meta: 거래일자, 공급자 사업자등록번호(profile.businessRegistrationNumber), 인수자 확인
- items: 품목 1행 이상, 품명·규격·수량·단가는 요청자 제공 값만 사용
- items.totals: 공급가액 합계 → 세액 합계 → 총 합계 순서
- signature: 발행일 + "회사명 대표이사 대표자명", stamp true

## Document JSON skeleton

```json
{
  "title": "거 래 명 세 서",
  "documentNumber": "T-<YYYYMMDD>-<순번>",
  "profile": { ...company profile... },
  "recipient": { "label": "공급받는자", "lines": ["<공급받는자 회사명>", "<담당자명> 님"] },
  "meta": [
    { "label": "거래일자", "value": "<YYYY-MM-DD>" },
    { "label": "사업자등록번호", "value": "<profile.businessRegistrationNumber>" },
    { "label": "인수자 확인", "value": "<인수자명 또는 서명란>" }
  ],
  "items": {
    "headers": ["품명", "규격", "수량", "단가", "공급가액", "세액"],
    "aligns": ["L", "L", "R", "R", "R", "R"],
    "rows": [["<품명>", "<규격>", "<수량>", "<단가>", "<공급가액>", "<세액>"]],
    "totals": [
      { "label": "공급가액 합계", "value": "<금액>원" },
      { "label": "세액 합계", "value": "<금액>원" },
      { "label": "총 합계 (부가세 포함)", "value": "<금액>원" }
    ]
  },
  "notes": ["위와 같이 거래명세서를 발행합니다."],
  "signature": { "date": "<YYYY년 M월 D일>", "line": "<회사명> 대표이사 <대표자명>", "stamp": true },
  "footer": "본 명세서는 실제 거래 내역과 일치함을 확인합니다."
}
```

## Fixed wording

- notes: "위와 같이 거래명세서를 발행합니다."
- 인수자 확인은 meta 또는 notes 중 요청자가 지정한 위치에 넣는다.

## Rules

- 금액은 천단위 콤마, 합계 값에만 "원"을 붙인다. 부가세 별도·포함을 반드시 명시한다.
- 세액은 공급가액의 10%로 계산하고 합계가 행 합과 일치하는지 검산한다.
- 공급자·공급받는자 명, 품명·단가·수량·거래일자가 없으면 지어내지 말고 요청자에게 확인한다.
- title은 "거 래 명 세 서"처럼 글자 사이 공백을 넣는다.
