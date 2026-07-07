# 근로계약서 (Employment Contract)

output: docx
filename: 근로계약서_<근로자명>_<YYYYMMDD>.docx

## Purpose

사용자("갑")와 근로자("을") 사이에 근로조건을 확정하는 편집 가능한 법정 문서. 근로기준법 제17조에 따른 필수 기재사항을 빠짐없이 포함해야 한다.

## Required fields

- 갑(회사): 상호, 주소, 대표자
- 을(근로자): 성명, 주소, 연락처(있는 경우)
- 계약기간, 근무장소 및 업무
- 소정근로시간, 휴게시간
- 임금의 구성·계산방법·지급방법(지급일, 지급방법)
- 휴일, 연차유급휴가
- 사회보험 적용 여부

## Document JSON skeleton

```json
{
  "title": "근로계약서",
  "fontName": "Noto Sans CJK KR",
  "fontSize": 10.5,
  "page": { "marginInches": 0.9 },
  "blocks": [
    { "type": "paragraph", "text": "<갑 상호>(이하 \"갑\"이라 한다)와(과) <을 성명>(이하 \"을\"이라 한다)은 다음과 같이 근로계약을 체결한다." },
    { "type": "heading", "level": 2, "text": "제1조 (계약기간)" },
    { "type": "paragraph", "text": "계약기간은 <YYYY-MM-DD>부터 <YYYY-MM-DD 또는 기간의 정함이 없음>까지로 한다." },
    { "type": "heading", "level": 2, "text": "제2조 (근무장소 및 업무)" },
    { "type": "paragraph", "text": "근무장소: <근무장소>" },
    { "type": "paragraph", "text": "담당업무: <담당업무>" },
    { "type": "heading", "level": 2, "text": "제3조 (근로시간 및 휴게시간)" },
    { "type": "paragraph", "text": "소정근로시간은 <시작시각>부터 <종료시각>까지로 하며, 1일 <시간>시간, 1주 <시간>시간으로 한다." },
    { "type": "paragraph", "text": "휴게시간은 근무시간 중 <휴게시간>으로 한다." },
    { "type": "heading", "level": 2, "text": "제4조 (임금)" },
    {
      "type": "table",
      "columnWidthsInches": [1.8, 4.2],
      "rows": [
        ["구성 항목", "금액/내용"],
        ["기본급", "<기본급>"],
        ["제수당", "<제수당 내역 또는 해당 없음>"],
        ["지급일", "매월 <일자>일"],
        ["지급방법", "<근로자 명의 계좌 직접 이체 등>"]
      ]
    },
    { "type": "paragraph", "text": "임금 계산방법: <시급/월급 및 계산 기준>" },
    { "type": "heading", "level": 2, "text": "제5조 (휴일 및 휴가)" },
    { "type": "paragraph", "text": "휴일은 <주휴일 및 공휴일 규정>에 따른다." },
    { "type": "paragraph", "text": "연차유급휴가는 근로기준법 제60조에 따라 부여한다." },
    { "type": "heading", "level": 2, "text": "제6조 (사회보험)" },
    { "type": "paragraph", "text": "갑은 을에 대하여 국민연금, 건강보험, 고용보험, 산재보험을 관계 법령에 따라 적용한다." },
    { "type": "heading", "level": 2, "text": "제7조 (비밀유지)" },
    { "type": "paragraph", "text": "을은 근무 중 및 퇴직 후에도 갑의 영업상·기술상 비밀을 제3자에게 누설하여서는 아니 된다." },
    { "type": "heading", "level": 2, "text": "제8조 (계약해지)" },
    { "type": "paragraph", "text": "갑과 을은 관계 법령 및 취업규칙이 정하는 사유와 절차에 따라 본 계약을 해지할 수 있다." },
    { "type": "heading", "level": 2, "text": "제9조 (기타)" },
    { "type": "paragraph", "text": "본 계약에서 정하지 아니한 사항은 근로기준법 등 관계 법령 및 갑의 취업규칙에 따른다." },
    { "type": "paragraph", "text": "본 계약을 증명하기 위하여 계약서 2부를 작성하여 갑·을이 서명 또는 날인 후 각 1부씩 보관한다." },
    { "type": "paragraph", "text": "<YYYY년 M월 D일>" },
    { "type": "heading", "level": 2, "text": "갑" },
    {
      "type": "table",
      "columnWidthsInches": [1.5, 4.5],
      "rows": [
        ["상호", "<갑 상호>"],
        ["주소", "<갑 주소>"],
        ["대표자", "<대표자명>  (인)"]
      ]
    },
    { "type": "heading", "level": 2, "text": "을" },
    {
      "type": "table",
      "columnWidthsInches": [1.5, 4.5],
      "rows": [
        ["성명", "<을 성명>  (서명)"],
        ["주소", "<을 주소>"]
      ]
    }
  ]
}
```

## Fixed wording

- 전문 문구, 제9조 마지막 문단("본 계약을 증명하기 위하여...")은 고정 문구로 항상 포함한다.
- 조 제목("제1조 (계약기간)" 등)의 번호와 제목은 위 순서를 기본으로 하되, 요청자가 조항을 추가·삭제하면 번호를 이어서 재정렬한다.

## Rules

- 임금 구성·계산방법·지급방법, 소정근로시간, 휴게, 휴일, 연차유급휴가, 근무장소와 업무는 근로기준법 제17조 필수 기재사항이므로 어떤 경우에도 생략하지 않는다.
- 이름, 급여, 날짜, 조건은 절대 지어내지 않는다. 필수 정보가 없으면 요청자에게 확인한다.
- 본 문서는 법률 검토 전 초안이며, 서명 전 최종 검토는 회사(전문 자격을 갖춘 담당자 또는 자문사)의 책임임을 인지하고 작성한다.
