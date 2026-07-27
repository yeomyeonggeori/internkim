# CRM v1 데이터베이스 스키마 제안

CRM v1에서 저장할 데이터, 기존 서비스와의 경계, 주요 제약조건을 정리한 설계 검토 문서다.

CRM은 관계처·연락처·진행 건·활동을 직접 관리한다. Flow·Calendar·Mail·Files는 기존 서비스가 원본을 유지하며 CRM은 외부 ID만 연결한다.

v1의 파일 입력과 출력은 CSV만 지원한다. Mail·Calendar에서 CRM 후보를 자동 생성하는 Intake Draft는 포함하지 않는다.

## 전체 테이블

CRM은 `crm.sqlite`를 단독으로 사용하므로 테이블 이름에 `crm_` 접두사를 붙이지 않는다. 기존 서비스 테이블과 같이 단수 이름을 쓴다.

| 구분 | 테이블 | 용도 |
|:---|:---|:---|
| 관계처 | `account` | 회사·기관·조직 정보 |
| 연락처 | `contact` | B2B 담당자와 B2C 개인 고객 |
| 진행 건 | `opportunity` | 영업·투자·후원·제휴 등 진행 건 |
| 활동 | `activity` | 통화·회의·메일·메모·단계 변경 등 활동 |
| 외부 연동 | `resource_link` | Flow·Calendar·Mail·Files 연결 |

## 키 표기

| 표기 | 의미 |
|:---|:---|
| `PK` | 테이블의 각 행을 구분하는 기본키 |
| `FK` | `crm.sqlite` 내부의 다른 테이블을 참조하는 외래키 |
| `논리 FK` | 다른 서비스가 관리하는 ID를 저장하지만 DB 외래키 제약은 걸지 않는 컬럼 |
| `UNIQUE` | 같은 값의 중복 저장을 금지하는 제약 |
| `CHECK` | enum, 날짜, 금액, 연결 조건 등 허용 범위를 검증하는 제약 |

## 시각과 날짜 형식

시간 값은 SQLite에 `TEXT`로 저장하되 형식을 하나로 고정한다.

| 종류 | 형식 | 예시 | 사용하는 컬럼 |
|:---|:---|:---|:---|
| 시각 | `YYYY-MM-DDTHH:MM:SSZ` | `2026-07-27T09:15:00Z` | `*_at` 로 끝나는 모든 컬럼 |
| 날짜 | `YYYY-MM-DD` | `2026-07-27` | `target_date` 등 `_date` 로 끝나는 컬럼 |

- RFC 3339 형식이며 항상 UTC로 저장한다. `Z` 이외의 오프셋은 저장하지 않는다.
- 소수점 이하 초는 저장하지 않는다. 자릿수가 고정되어야 문자열 정렬이 곧 시간 순서가 되고 `BETWEEN` 범위 조회가 정확해진다.
- 사용자 표시 시각의 시간대 변환은 조회 계층에서 처리한다.
- 저장 값에 `CHECK`를 걸어 형식을 강제한다.

```sql
CHECK (created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9]Z')
CHECK (target_date GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]')
```

이 형식은 SQLite의 `datetime()`, `julianday()`, `strftime()`이 그대로 인식한다.

금액은 부동소수점 대신 최소 화폐 단위의 정수로 저장한다.

## 공통 컬럼

보관 가능한 핵심 데이터 테이블은 다음 컬럼을 공통으로 사용한다.

| 컬럼 | 타입 | 필수 | 키 | 저장 내용 |
|:---|:---:|:---:|:---:|:---|
| `id` | `TEXT` | O | PK | UUID 또는 ULID |
| `created_at` | `TEXT` | O | CHECK | 생성 시각 |
| `created_by_person_id` | `TEXT` | O | 논리 FK | 생성한 내부 사용자 ID |
| `updated_at` | `TEXT` | O | CHECK | 마지막 수정 시각 |
| `updated_by_person_id` | `TEXT` | O | 논리 FK | 마지막 수정 사용자 ID |
| `archived_at` | `TEXT` | X | CHECK | 보관 처리 시각 |
| `archived_by_person_id` | `TEXT` | X | 논리 FK | 보관 처리 사용자 ID |

## 관계처

### `account`

유형과 태그는 별도 조인 테이블 대신 JSON 배열 컬럼에 저장한다. 검색은 `json_each`로 처리한다.

| 컬럼 | 타입 | 필수 | 키 | 저장 내용 |
|:---|:---:|:---:|:---:|:---|
| `id` | `TEXT` | O | PK | 관계처 ID |
| `name` | `TEXT` | O |  | 회사·기관·조직 이름 |
| `status` | `TEXT` | O | CHECK | `active`, `warm`, `cold`, `paused`, `closed` |
| `types_json` | `TEXT` | O |  | `customer`, `partner`, `sponsor`, `vendor`, `investor`, `other` 중 복수 선택 |
| `tags_json` | `TEXT` | O |  | 검색·분류용 태그 배열 |
| `importance` | `TEXT` | O | CHECK | `high`, `medium`, `low` |
| `owner_person_id` | `TEXT` | O | 논리 FK | 내부 담당자 ID |
| `owner_circle_id` | `TEXT` | X | 논리 FK | 담당 팀·조직 ID |
| `address` | `TEXT` | X |  | 주소 |
| `description` | `TEXT` | X |  | 비고와 설명 |
| 공통 컬럼 |  |  |  | 생성·수정·보관 정보 |

## 연락처

### `contact`

B2B 담당자와 B2C 개인 고객을 함께 저장한다. 소속이 없는 개인 고객은 `account_id`를 비운다.

v1은 한 사람이 하나의 관계처에 소속된다고 본다. 소속 정보를 별도 조인 테이블로 두지 않고 이 테이블에 직접 저장한다.

| 컬럼 | 타입 | 필수 | 키 | 저장 내용 |
|:---|:---:|:---:|:---:|:---|
| `id` | `TEXT` | O | PK | 연락처 ID |
| `account_id` | `TEXT` | X | FK | 소속 관계처 ID |
| `name` | `TEXT` | O |  | 이름 |
| `email` | `TEXT` | O |  | 이메일 |
| `phone` | `TEXT` | O |  | 전화번호 |
| `title` | `TEXT` | X |  | 직함·역할 |
| `department` | `TEXT` | X |  | 부서 |
| `is_primary` | `INTEGER` | O | CHECK | 소속 관계처의 주 담당자 여부 |
| `owner_person_id` | `TEXT` | O | 논리 FK | 내부 담당자 ID |
| `owner_circle_id` | `TEXT` | X | 논리 FK | 담당 팀·조직 ID |
| `note` | `TEXT` | X |  | 비고 |
| 공통 컬럼 |  |  |  | 생성·수정·보관 정보 |

이메일과 전화번호는 모두 필수다. 중복 검색용 정규화 컬럼을 따로 두지 않고 표현식 인덱스로 처리한다.

```sql
CREATE INDEX contact_email_lookup ON contact (lower(trim(email)));
CREATE INDEX contact_phone_lookup ON contact (replace(replace(replace(phone, '-', ''), ' ', ''), '+82', '0'));
```

같은 값이 발견되어도 자동 병합하지 않고 사용자 검토 대상으로 표시한다.

## 진행 건

### `opportunity`

진행 건은 관계처, 연락처 또는 둘 모두에 연결할 수 있다. 둘 모두 연결하면 해당 연락처의 `account_id`와 일치해야 한다.

| 컬럼 | 타입 | 필수 | 키 | 저장 내용 |
|:---|:---:|:---:|:---:|:---|
| `id` | `TEXT` | O | PK | 진행 건 ID |
| `account_id` | `TEXT` | X | FK | 관련 관계처 ID |
| `contact_id` | `TEXT` | X | FK | 관련 연락처 ID |
| `business_key` | `TEXT` | O | 논리 FK | 사업 식별자 |
| `name` | `TEXT` | O |  | 진행 건 이름 |
| `kind` | `TEXT` | O | CHECK | `sales`, `investment`, `sponsorship`, `partnership`, `procurement` |
| `stage` | `TEXT` | O | CHECK | 현재 파이프라인 단계 |
| `stage_position` | `REAL` | O |  | 같은 단계 내 수동 카드 정렬 순서 |
| `stage_changed_at` | `TEXT` | O | CHECK | 현재 단계로 이동한 시각 |
| `owner_person_id` | `TEXT` | O | 논리 FK | 내부 담당자 ID |
| `owner_circle_id` | `TEXT` | X | 논리 FK | 담당 팀·조직 ID |
| `expected_amount_minor` | `INTEGER` | X | CHECK | 예상 금액의 최소 화폐 단위 |
| `currency_code` | `TEXT` | O | CHECK | `KRW`, `USD`, `JPY`, `EUR` |
| `importance` | `TEXT` | O | CHECK | `high`, `medium`, `low` |
| `target_date` | `TEXT` | X | CHECK | 목표일 |
| `description` | `TEXT` | X |  | 진행 건 설명 |
| 공통 컬럼 |  |  |  | 생성·수정·보관 정보 |

파이프라인 단계의 기본 순서는 `lead`, `qualified`, `proposal`, `negotiation`, `won`, `lost`, `on_hold` 이다.

`stage_position`은 실수로 저장하고 카드를 옮기면 앞뒤 카드 값의 중간값을 넣는다. 정수 간격 재배정 로직은 두지 않는다.

단계 변경 이력은 `activity`에 `kind = 'stage_change'` 로 기록한다. 별도 이력 테이블을 두지 않는다.

## 활동

### `activity`

| 컬럼 | 타입 | 필수 | 키 | 저장 내용 |
|:---|:---:|:---:|:---:|:---|
| `id` | `TEXT` | O | PK | 활동 ID |
| `account_id` | `TEXT` | X | FK | 관련 관계처 ID |
| `contact_id` | `TEXT` | X | FK | 관련 연락처 ID |
| `opportunity_id` | `TEXT` | X | FK | 관련 진행 건 ID |
| `kind` | `TEXT` | O | CHECK | `note`, `email`, `meeting`, `call`, `task`, `file`, `event`, `stage_change` |
| `title` | `TEXT` | O |  | 활동 제목 |
| `occurred_at` | `TEXT` | O | CHECK | 활동 발생 시각 |
| `summary` | `TEXT` | X |  | 활동 내용 |
| 공통 컬럼 |  |  |  | 생성·수정·보관 정보 |

활동은 관계처·연락처·진행 건 중 하나 이상에 연결해야 한다. 진행 건에 연결하면 `account_id`, `contact_id`는 진행 건의 값을 따르며 서로 충돌하는 값을 저장할 수 없다.

Calendar 행사 참여는 `kind = 'event'` 활동과 `resource_link`의 Calendar 연결로 표현한다.

## 외부 연동

### `resource_link`

CRM 레코드 하나에 연결할 수 있는 외부 자원 개수에는 제한을 두지 않는다.

| 컬럼 | 타입 | 필수 | 키 | 저장 내용 |
|:---|:---:|:---:|:---:|:---|
| `id` | `TEXT` | O | PK | 연결 ID |
| `entity_type` | `TEXT` | O | CHECK | `account`, `contact`, `opportunity`, `activity` |
| `entity_id` | `TEXT` | O | 논리 FK | 연결할 CRM 레코드 ID |
| `service` | `TEXT` | O | CHECK | `flow`, `calendar`, `mail`, `files` |
| `resource_type` | `TEXT` | O |  | `task`, `event`, `message`, `file` 등 서비스 자원 종류 |
| `external_resource_id` | `TEXT` | O | 논리 FK | 외부 서비스의 실제 ID |
| `external_url` | `TEXT` | X |  | 외부 자원 바로가기 URL |
| `created_at` | `TEXT` | O | CHECK | 연결 생성 시각 |
| `created_by_person_id` | `TEXT` | O | 논리 FK | 연결 생성 사용자 ID |
| `removed_at` | `TEXT` | X | CHECK | 연결 해제 시각 |
| `removed_by_person_id` | `TEXT` | X | 논리 FK | 연결 해제 사용자 ID |

외부 자원 생성은 요청 시점에 동기로 호출하고 실패하면 사용자에게 오류를 표시한다. v1에 outbox 큐, 워커 lease, 재시도 스케줄러는 두지 않는다.

## CSV 가져오기와 내보내기

가져오기는 업로드 → 미리보기 → 반영 세 단계로 처리하며 진행 상태를 DB에 저장하지 않는다.

1. 업로드한 CSV를 파싱해 검증 결과, 중복 후보, 컬럼 매핑을 응답으로 돌려준다.
2. 사용자가 행별로 `기존 정보 업데이트`, `새 연락처로 생성`, `건너뛰기` 중 하나를 고른다. 이메일과 전화번호가 모두 같은 기존 연락처를 가리키면 `기존 정보 업데이트`를 기본 선택으로 보여준다. 이메일과 전화번호가 서로 다른 기존 연락처를 가리키면 충돌로 표시하고 자동 처리하지 않는다.
3. 확정된 내용을 한 트랜잭션으로 반영한다.

내보내기는 `GET /crm/export.csv?entity=...&filters=...` 로 결과를 바로 스트리밍한다. 파일을 Files에 저장하거나 만료 시각을 관리하지 않는다.

CSV 원본은 어느 단계에서도 CRM DB에 보관하지 않는다.

## 권한

| 작업 | 허용 대상 |
|:---|:---|
| CRM 데이터 조회 | 모든 내부 사용자 |
| 관계처·연락처·진행 건·활동 생성·수정 | 레코드 담당자 또는 담당 팀 구성원 |
| 담당자·담당 팀 변경 | 레코드 수정 권한이 있는 사용자 |
| CSV 가져오기 | 레코드 생성·수정 권한이 있는 내부 사용자 |
| CSV 내보내기 | 관리자 |
| 레코드 보관·복원 | 관리자 |

담당 팀 권한은 기존 조직도의 `circle_id`와 구성원 정보를 기준으로 판단한다. 활동은 연결된 진행 건, 관계처 또는 연락처의 담당자·담당 팀 권한을 따른다. CSV 가져오기로 기존 레코드를 수정할 때도 각 레코드의 수정 권한을 확인한다. 보관은 소프트 삭제로 처리하며 v1에서는 영구 삭제 기능을 제공하지 않는다.

변경 이력은 각 테이블의 `created_by_person_id`, `updated_by_person_id`, `archived_by_person_id`로 남긴다. 필드 단위 감사 로그 테이블은 v1에 두지 않는다.

## 저장하지 않거나 원본을 복제하지 않는 값

| 화면에 보이는 값 | 처리 방식 |
|:---|:---|
| 내부 담당자 이름·이메일·팀 이름 | `person_id`, `circle_id`로 기존 조직도에서 조회 |
| 마지막 접촉일 | 가장 최근 활동 시각으로 계산 |
| 다음 후속 조치일 | 연결된 Flow 업무에서 조회 |
| 진행 중인 건수 | 종료되지 않은 진행 건 개수로 계산 |
| 예상 금액 합계 | 진행 건 금액을 통화별로 합산 |
| 단계 경과 기간 | `stage_changed_at`부터 계산 |
| 사업 표시 이름 | `business_key`로 기존 사업 정의에서 조회 |
| Flow 업무 상태·담당자·기한 | Flow가 원본으로 관리 |
| Calendar 행사 제목·시간·장소 | Calendar가 원본으로 관리 |
| 메일 본문과 첨부파일 | Mail과 Files가 원본으로 관리 |
| 구독과 결제 정보 | 상품·결제 서비스가 원본으로 관리하며 CRM은 복제하지 않음 |
| KPI와 파이프라인 집계 | CRM 원본 데이터에서 계산 |
| CSV 원본과 내보내기 결과 파일 | CRM DB에 보관하지 않음 |
| Intake Draft | v1에서는 생성하거나 저장하지 않음 |

## 주요 제약조건

| 테이블 | 제약조건 |
|:---|:---|
| 모든 테이블 | `*_at` 은 `YYYY-MM-DDTHH:MM:SSZ`, `*_date` 는 `YYYY-MM-DD` 형식을 `CHECK`로 강제 |
| `contact` | 이름·이메일·전화번호 필수, 이메일·전화번호 표현식 인덱스 |
| `contact` | 관계처별 보관되지 않은 주 담당자는 최대 한 명 |
| `opportunity` | 관계처·연락처 중 하나 이상 필수, 둘 다 있으면 연락처의 `account_id`와 일치 |
| `opportunity` | `(stage, stage_position)` 인덱스, 단계 이동 시 `stage_changed_at` 갱신과 활동 기록 생성 |
| `activity` | 관계처·연락처·진행 건 중 하나 이상 필수, 진행 건 연결 값과 불일치 금지 |
| `resource_link` | 연결 개수는 무제한, 같은 CRM 레코드와 외부 자원의 활성 중복 연결 금지 |
| 주요 테이블 | enum, 금액, 날짜, 불리언 값에 `CHECK` 적용 |

## v1에서 제외한 것과 도입 시점

| 제외한 것 | 도입 시점 |
|:---|:---|
| 관계처 유형·태그 조인 테이블 | 태그별 집계 질의가 JSON 검색으로 느려질 때 |
| 다중 소속 조인 테이블 | 겸직·이직 이력을 조회하는 화면이 생길 때 |
| 단계 변경 이력 테이블 | 활동 기록만으로 파이프라인 분석이 부족할 때 |
| 구독 테이블 | 상품 서비스 조회로 감당이 안 될 때 |
| 행사 참여 테이블 | 참여 상태별 집계가 실제 요구로 올라올 때 |
| 연동 outbox 큐 | 외부 호출 실패율이 사용자에게 문제가 될 때 |
| 가져오기·내보내기 작업 테이블 | CSV 규모가 커져 동기 처리가 시간 초과할 때 |
| 감사 로그 테이블 | 필드 단위 이력 요구가 문서로 확정될 때 |

보관된 레코드는 v1에서 자동 삭제하지 않는다. 주기적 정리 작업도 두지 않는다.
