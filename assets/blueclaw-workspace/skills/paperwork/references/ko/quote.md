# 견적서 (Quotation)

output: pdf
filename: 견적서_<수신처>_<YYYYMMDD>.pdf

## Purpose

거래처에 상품·서비스 가격을 제안하는 대외 문서. 수신처와 품목·금액이 확정 정보여야 한다.

## Required fields

- recipient: 수신처 회사명 필수, 담당자명은 있으면 함께
- meta: 견적일자, 유효기간, 입금계좌(profile의 bankAccount)
- items: 품목 1행 이상, 단가·수량은 요청자 제공 값만 사용
- items.totals: 공급가액 합계 → 부가세(10%) → 총 합계 순서
- signature: 발행일 + "회사명 대표이사 대표자명", stamp true

## Document JSON skeleton

```json
{
  "title": "견 적 서",
  "documentNumber": "Q-<YYYYMMDD>-<순번>",
  "profile": { ...company profile... },
  "recipient": { "label": "수신", "lines": ["<수신처 회사명>", "<담당자명> 님"] },
  "meta": [
    { "label": "견적일자", "value": "<YYYY-MM-DD>" },
    { "label": "유효기간", "value": "발행일로부터 30일" },
    { "label": "입금계좌", "value": "<profile.bankAccount>" }
  ],
  "items": {
    "headers": ["품명", "규격", "수량", "단가", "공급가액", "세액"],
    "aligns": ["L", "L", "R", "R", "R", "R"],
    "rows": [["<품명>", "<규격>", "<수량>", "<단가>", "<공급가액>", "<세액>"]],
    "totals": [
      { "label": "공급가액 합계", "value": "<금액>원" },
      { "label": "부가세(10%)", "value": "<금액>원" },
      { "label": "총 합계 (부가세 포함)", "value": "<금액>원" }
    ]
  },
  "notes": ["위와 같이 견적합니다."],
  "signature": { "date": "<YYYY년 M월 D일>", "line": "<회사명> 대표이사 <대표자명>", "stamp": true },
  "footer": "본 견적서는 발행일로부터 30일간 유효합니다."
}
```

## Fixed wording

- notes: "위와 같이 견적합니다."
- footer: 유효기간 문구. 요청자가 다른 유효기간을 주면 meta와 footer를 함께 맞춘다.

## Rules

- 금액은 천단위 콤마, 합계 값에만 "원"을 붙인다. 부가세 별도·포함을 반드시 명시한다.
- 세액은 공급가액의 10%로 계산하고 합계가 행 합과 일치하는지 검산한다.
- 품명·단가·수량이 없으면 지어내지 말고 요청자에게 확인한다.
- title은 "견 적 서"처럼 글자 사이 공백을 넣는다.
