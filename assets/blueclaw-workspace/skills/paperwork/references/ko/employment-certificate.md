# 재직증명서 (Certificate of Employment)

output: pdf
filename: 재직증명서_<성명>_<YYYYMMDD>.pdf

## Purpose

현재 재직 중인 임직원의 재직 사실을 증명하는 대내외 제출용 문서. 제출처(용도)가 있는 경우가 많으므로 확인한다.

## Required fields

- meta: 성명, 생년월일, 소속, 직위, 재직기간, 용도
- 재직기간은 입사일 ~ "현재" 형식으로 표기 (아직 재직 중이므로 종료일이 없음)
- signature: 발행일 + "회사명 대표이사 대표자명", stamp true

## Document JSON skeleton

```json
{
  "title": "재 직 증 명 서",
  "documentNumber": "EC-<YYYYMMDD>-<순번>",
  "profile": { ...company profile... },
  "meta": [
    { "label": "성명", "value": "<성명>" },
    { "label": "생년월일", "value": "<YYYY-MM-DD>" },
    { "label": "소속", "value": "<부서명>" },
    { "label": "직위", "value": "<직위>" },
    { "label": "재직기간", "value": "<입사일> ~ 현재" },
    { "label": "용도", "value": "<제출처 또는 용도>" }
  ],
  "notes": ["위 사람은 당사에 재직하고 있음을 증명합니다."],
  "signature": { "date": "<YYYY년 M월 D일>", "line": "<회사명> 대표이사 <대표자명>", "stamp": true },
  "footer": "본 증명서는 발급일 기준 재직 사실을 증명합니다."
}
```

## Fixed wording

- notes: "위 사람은 당사에 재직하고 있음을 증명합니다."
- 재직기간의 종료 시점은 항상 "현재"로 고정한다 (재직증명서는 퇴사자에게 발급하지 않는다).

## Rules

- 성명, 생년월일, 소속, 직위, 입사일, 용도가 없으면 지어내지 말고 요청자에게 확인한다.
- 급여·계약조건 등 재직 사실 증명과 무관한 정보는 포함하지 않는다.
- title은 "재 직 증 명 서"처럼 글자 사이 공백을 넣는다.
