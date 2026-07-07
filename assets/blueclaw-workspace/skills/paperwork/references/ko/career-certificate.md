# 경력증명서 (Certificate of Career)

output: pdf
filename: 경력증명서_<성명>_<YYYYMMDD>.pdf

## Purpose

퇴사자 또는 근무 이력이 있는 사람의 과거 근무 사실을 증명하는 대내외 제출용 문서. 재직증명서와 달리 재직기간이 종료된 확정 기간이다.

## Required fields

- meta: 성명, 생년월일, 소속, 직위, 담당업무, 재직기간, 용도
- 재직기간은 입사일 ~ 퇴사일 형식의 확정된 기간으로 표기 (재직증명서처럼 "현재"를 쓰지 않는다)
- signature: 발행일 + "회사명 대표이사 대표자명", stamp true

## Document JSON skeleton

```json
{
  "title": "경 력 증 명 서",
  "documentNumber": "CC-<YYYYMMDD>-<순번>",
  "profile": { ...company profile... },
  "meta": [
    { "label": "성명", "value": "<성명>" },
    { "label": "생년월일", "value": "<YYYY-MM-DD>" },
    { "label": "소속", "value": "<부서명>" },
    { "label": "직위", "value": "<직위>" },
    { "label": "담당업무", "value": "<담당업무>" },
    { "label": "재직기간", "value": "<입사일> ~ <퇴사일>" },
    { "label": "용도", "value": "<제출처 또는 용도>" }
  ],
  "notes": ["위 사람은 당사에서 위와 같이 근무하였음을 증명합니다."],
  "signature": { "date": "<YYYY년 M월 D일>", "line": "<회사명> 대표이사 <대표자명>", "stamp": true },
  "footer": "본 증명서는 상기 근무 이력이 사실임을 증명합니다."
}
```

## Fixed wording

- notes: "위 사람은 당사에서 위와 같이 근무하였음을 증명합니다."
- 재직기간은 반드시 종료일이 있는 닫힌 기간으로 표기한다.

## Rules

- 성명, 생년월일, 소속, 직위, 담당업무, 입사일, 퇴사일, 용도가 없으면 지어내지 말고 요청자에게 확인한다.
- 퇴사 사유·평가·급여 등 근무 이력 증명과 무관한 정보는 포함하지 않는다.
- title은 "경 력 증 명 서"처럼 글자 사이 공백을 넣는다.
