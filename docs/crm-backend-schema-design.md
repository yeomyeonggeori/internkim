# CRM v1 데이터베이스 스키마 제안

CRM v1에서 저장할 데이터, 기존 서비스와의 경계, 주요 제약조건을 정리한 설계 검토 문서다.

CRM은 관계처·사람·진행 건·활동·구독·행사 참여를 직접 관리한다. Flow·Calendar·Mail·Files는 기존 서비스가 원본을 유지하며 CRM은 외부 ID만 연결한다.

v1의 파일 입력과 출력은 CSV만 지원한다. Mail·Calendar에서 CRM 후보를 자동 생성하는 Intake Draft는 포함하지 않는다.

## 전체 테이블

| 구분 | 테이블 | 용도 |
|:---|:---|:---|
| 관계처 | `crm_accounts` | 회사·기관·조직 정보 |
| 관계처 | `crm_account_type_assignments` | 관계처의 복수 유형 |
| 관계처 | `crm_account_tags` | 관계처 태그 |
| 사람 | `crm_people` | B2B 담당자와 B2C 개인 고객 |
| 사람 | `crm_account_contacts` | 사람과 관계처의 복수 소속 관계 |
| 진행 건 | `crm_opportunities` | 영업·투자·후원·제휴 등 진행 건 |
| 진행 건 | `crm_opportunity_stage_changes` | 진행 단계 변경 이력 |
| 활동 | `crm_activities` | 통화·회의·메일·메모 등 활동 |
| 구독 | `crm_subscriptions` | 사람의 서비스 계정별 구독 |
| 행사 | `crm_event_participations` | 사람의 Calendar 행사 참여 |
| 외부 연동 | `crm_resource_links` | Flow·Calendar·Mail·Files 연결 |
| 외부 연동 | `crm_integration_outbox` | 외부 생성·수정 요청과 재시도 |
| 가져오기 | `crm_import_jobs` | CSV 가져오기 작업 |
| 가져오기 | `crm_import_rows` | CSV 행별 검증·중복 검토 결과 |
| 가져오기 | `crm_import_row_results` | 한 CSV 행에서 생성·수정된 복수 레코드 |
| 내보내기 | `crm_export_jobs` | CSV 생성 작업 |
| 감사 | `crm_audit_events` | 주요 데이터 변경과 관리 작업 이력 |

## 키 표기

| 표기 | 의미 |
|:---|:---|
| `PK` | 테이블의 각 행을 구분하는 기본키 |
| `FK` | `crm.sqlite` 내부의 다른 테이블을 참조하는 외래키 |
| `논리 FK` | 다른 서비스가 관리하는 ID를 저장하지만 DB 외래키 제약은 걸지 않는 컬럼 |
| `UNIQUE` | 같은 값의 중복 저장을 금지하는 제약 |
| `CHECK` | enum, 날짜, 금액, 연결 조건 등 허용 범위를 검증하는 제약 |

## 공통 컬럼

보관 가능한 핵심 데이터 테이블은 다음 컬럼을 공통으로 사용한다.

| 컬럼 | 타입 | 필수 | 키 | 저장 내용 |
|:---|:---:|:---:|:---:|:---|
| `id` | `TEXT` | O | PK | UUID 또는 ULID |
| `created_at` | `TEXT` | O |  | 생성 시각 |
| `created_by_person_id` | `TEXT` | O | 논리 FK | 생성한 내부 사용자 ID |
| `updated_at` | `TEXT` | O |  | 마지막 수정 시각 |
| `updated_by_person_id` | `TEXT` | O | 논리 FK | 마지막 수정 사용자 ID |
| `archived_at` | `TEXT` | X |  | 보관 처리 시각 |
| `archived_by_person_id` | `TEXT` | X | 논리 FK | 보관 처리 사용자 ID |

시간은 UTC ISO 8601 문자열로 저장한다. 금액은 부동소수점 대신 최소 화폐 단위의 정수로 저장한다.

## 관계처

### `crm_accounts`

| 컬럼 | 타입 | 필수 | 키 | 저장 내용 |
|:---|:---:|:---:|:---:|:---|
| `id` | `TEXT` | O | PK | 관계처 ID |
| `name` | `TEXT` | O |  | 회사·기관·조직 이름 |
| `status` | `TEXT` | O | CHECK | `active`, `warm`, `cold`, `paused`, `closed` |
| `importance` | `TEXT` | O | CHECK | `high`, `medium`, `low` |
| `owner_person_id` | `TEXT` | O | 논리 FK | 내부 담당자 ID |
| `owner_circle_id` | `TEXT` | X | 논리 FK | 담당 팀·조직 ID |
| `address` | `TEXT` | X |  | 주소 |
| `description` | `TEXT` | X |  | 비고와 설명 |
| 공통 컬럼 |  |  |  | 생성·수정·보관 정보 |

### `crm_account_type_assignments`

| 컬럼 | 타입 | 필수 | 키 | 저장 내용 |
|:---|:---:|:---:|:---:|:---|
| `account_id` | `TEXT` | O | PK, FK | 관계처 ID |
| `account_type` | `TEXT` | O | PK, CHECK | `customer`, `partner`, `sponsor`, `vendor`, `investor`, `other` |
| `created_at` | `TEXT` | O |  | 유형 지정 시각 |
| `created_by_person_id` | `TEXT` | O | 논리 FK | 지정한 내부 사용자 ID |

### `crm_account_tags`

| 컬럼 | 타입 | 필수 | 키 | 저장 내용 |
|:---|:---:|:---:|:---:|:---|
| `account_id` | `TEXT` | O | PK, FK | 관계처 ID |
| `tag` | `TEXT` | O | PK | 검색·분류용 태그 |
| `created_at` | `TEXT` | O |  | 태그 등록 시각 |
| `created_by_person_id` | `TEXT` | O | 논리 FK | 등록한 내부 사용자 ID |

## 사람

### `crm_people`

관계처와 무관하게 한 사람을 한 번 저장한다. 특정 회사에 소속되지 않은 B2C 고객도 이 테이블에서 관리한다.

| 컬럼 | 타입 | 필수 | 키 | 저장 내용 |
|:---|:---:|:---:|:---:|:---|
| `id` | `TEXT` | O | PK | 사람 ID |
| `name` | `TEXT` | O |  | 이름 |
| `email` | `TEXT` | O |  | 표시용 이메일 |
| `normalized_email` | `TEXT` | O | INDEX | 공백·대소문자를 정규화한 중복 검색용 이메일 |
| `phone` | `TEXT` | O |  | 표시용 전화번호 |
| `normalized_phone` | `TEXT` | O | INDEX | 국가번호·구분 문자를 정규화한 중복 검색용 전화번호 |
| `owner_person_id` | `TEXT` | O | 논리 FK | 내부 담당자 ID |
| `owner_circle_id` | `TEXT` | X | 논리 FK | 담당 팀·조직 ID |
| `note` | `TEXT` | X |  | 사람에 대한 공통 비고 |
| 공통 컬럼 |  |  |  | 생성·수정·보관 정보 |

이메일과 전화번호는 모두 필수다. 정규화 값은 중복 후보를 찾는 용도이며 `UNIQUE`로 강제하지 않는다. 같은 값이 발견되어도 자동 병합하지 않고 사용자 검토 대상으로 표시한다.

### `crm_account_contacts`

한 사람이 여러 관계처에 소속될 수 있으므로 사람의 직함·부서·주 담당자 여부는 소속 관계에 저장한다.

| 컬럼 | 타입 | 필수 | 키 | 저장 내용 |
|:---|:---:|:---:|:---:|:---|
| `account_id` | `TEXT` | O | PK, FK | 관계처 ID |
| `person_id` | `TEXT` | O | PK, FK | 사람 ID |
| `title` | `TEXT` | X |  | 해당 관계처에서의 직함·역할 |
| `department` | `TEXT` | X |  | 해당 관계처에서의 부서 |
| `is_primary` | `INTEGER` | O | CHECK | 관계처의 주 담당자 여부 |
| `started_at` | `TEXT` | X |  | 소속 시작일 |
| `ended_at` | `TEXT` | X |  | 소속 종료일 |
| `note` | `TEXT` | X |  | 해당 소속에 대한 비고 |
| `created_at` | `TEXT` | O |  | 연결 생성 시각 |
| `created_by_person_id` | `TEXT` | O | 논리 FK | 연결 생성 사용자 ID |
| `updated_at` | `TEXT` | O |  | 마지막 수정 시각 |
| `updated_by_person_id` | `TEXT` | O | 논리 FK | 마지막 수정 사용자 ID |

## 진행 건

### `crm_opportunities`

진행 건은 관계처, 사람 또는 둘 모두에 연결할 수 있다. 둘 모두 연결할 경우 해당 사람과 관계처의 `crm_account_contacts` 소속 관계가 먼저 존재해야 한다.

| 컬럼 | 타입 | 필수 | 키 | 저장 내용 |
|:---|:---:|:---:|:---:|:---|
| `id` | `TEXT` | O | PK | 진행 건 ID |
| `account_id` | `TEXT` | X | FK | 관련 관계처 ID |
| `person_id` | `TEXT` | X | FK | 관련 사람 ID |
| `business_key` | `TEXT` | O | 논리 FK | 사업 식별자 |
| `name` | `TEXT` | O |  | 진행 건 이름 |
| `kind` | `TEXT` | O | CHECK | `sales`, `investment`, `sponsorship`, `partnership`, `procurement` |
| `stage` | `TEXT` | O | CHECK | 현재 파이프라인 단계 |
| `stage_position` | `INTEGER` | O |  | 같은 단계 내 수동 카드 정렬 순서 |
| `owner_person_id` | `TEXT` | O | 논리 FK | 내부 담당자 ID |
| `owner_circle_id` | `TEXT` | X | 논리 FK | 담당 팀·조직 ID |
| `expected_amount_minor` | `INTEGER` | X | CHECK | 예상 금액의 최소 화폐 단위 |
| `currency_code` | `TEXT` | O | CHECK | `KRW`, `USD`, `JPY`, `EUR` |
| `importance` | `TEXT` | O | CHECK | `high`, `medium`, `low` |
| `target_date` | `TEXT` | X |  | 목표일 |
| `description` | `TEXT` | X |  | 진행 건 설명 |
| 공통 컬럼 |  |  |  | 생성·수정·보관 정보 |

파이프라인 단계의 기본 순서는 다음과 같다.

1. `lead`.
2. `qualified`.
3. `proposal`.
4. `negotiation`.
5. `won`.
6. `lost`.
7. `on_hold`.

`stage_position`은 1000 단위의 간격을 둔 정수로 저장한다. 사용자가 카드를 이동하면 같은 단계 안의 순서를 저장하고, 간격이 부족할 때만 해당 단계의 순서를 다시 배정한다.

### `crm_opportunity_stage_changes`

| 컬럼 | 타입 | 필수 | 키 | 저장 내용 |
|:---|:---:|:---:|:---:|:---|
| `id` | `TEXT` | O | PK | 단계 변경 이력 ID |
| `opportunity_id` | `TEXT` | O | FK | 진행 건 ID |
| `from_stage` | `TEXT` | X | CHECK | 이전 단계, 최초 생성이면 없음 |
| `to_stage` | `TEXT` | O | CHECK | 변경된 단계 |
| `changed_at` | `TEXT` | O |  | 변경 시각 |
| `changed_by_person_id` | `TEXT` | O | 논리 FK | 변경한 내부 사용자 ID |
| `reason` | `TEXT` | X |  | 변경 사유 |

## 활동

### `crm_activities`

| 컬럼 | 타입 | 필수 | 키 | 저장 내용 |
|:---|:---:|:---:|:---:|:---|
| `id` | `TEXT` | O | PK | 활동 ID |
| `account_id` | `TEXT` | X | FK | 관련 관계처 ID |
| `person_id` | `TEXT` | X | FK | 관련 사람 ID |
| `opportunity_id` | `TEXT` | X | FK | 관련 진행 건 ID |
| `business_key` | `TEXT` | X | 논리 FK | 관련 사업 식별자 |
| `kind` | `TEXT` | O | CHECK | `note`, `email`, `meeting`, `call`, `task`, `file`, `stage_change` |
| `title` | `TEXT` | O |  | 활동 제목 |
| `occurred_at` | `TEXT` | O |  | 활동 발생 시각 |
| `summary` | `TEXT` | X |  | 활동 내용 |
| 공통 컬럼 |  |  |  | 생성·수정·보관 정보 |

활동은 관계처·사람·진행 건 중 하나 이상에 연결해야 한다. 진행 건에 연결하면 `account_id`, `person_id`, `business_key`는 진행 건의 값을 따르며 서로 충돌하는 값을 저장할 수 없다.

## 구독

### `crm_subscriptions`

구독은 사람 단위가 아니라 서비스 계정 단위로 구분한다. 같은 사람이 여러 서비스 계정을 가지면 같은 상품의 활성 구독도 여러 개 저장할 수 있다.

| 컬럼 | 타입 | 필수 | 키 | 저장 내용 |
|:---|:---:|:---:|:---:|:---|
| `id` | `TEXT` | O | PK | 구독 ID |
| `person_id` | `TEXT` | O | FK | 구독을 사용하는 사람 ID |
| `account_id` | `TEXT` | X | FK | B2B 구독과 관련된 관계처 ID |
| `source_service` | `TEXT` | O |  | 구독 원본 서비스 식별자 |
| `product_key` | `TEXT` | O |  | 구독 상품 식별자 |
| `plan_key` | `TEXT` | X |  | 상품 내 요금제 식별자 |
| `service_account_id` | `TEXT` | O |  | 상품 서비스 안에서 사용하는 계정 ID |
| `external_subscription_id` | `TEXT` | X |  | 외부 구독 시스템의 구독 ID |
| `status` | `TEXT` | O | CHECK | `trial`, `active`, `paused`, `canceled`, `expired` |
| `started_at` | `TEXT` | O |  | 구독 시작 시각 |
| `current_period_ends_at` | `TEXT` | X |  | 현재 구독 기간 종료 예정 시각 |
| `ended_at` | `TEXT` | X |  | 해지·만료 시각 |
| `note` | `TEXT` | X |  | 구독 관련 비고 |
| 공통 컬럼 |  |  |  | 생성·수정·보관 정보 |

사람 상세 화면에서는 구독을 하나의 영역으로 묶되 각 서비스 계정의 구독을 별도 행으로 표시한다. 같은 `source_service`, `service_account_id`, `product_key` 조합에는 `trial`, `active`, `paused` 상태의 구독을 동시에 하나만 허용하고 종료된 이력은 계속 보관한다. `account_id`가 있으면 해당 사람과 관계처의 소속 관계가 존재해야 한다. 결제 수단, 카드 정보, 청구 원문은 CRM에 저장하지 않는다.

## 행사 참여

### `crm_event_participations`

행사의 제목·시간·장소 원본은 Calendar가 관리한다. CRM은 사람과 Calendar 행사 ID의 참여 관계만 저장한다.

| 컬럼 | 타입 | 필수 | 키 | 저장 내용 |
|:---|:---:|:---:|:---:|:---|
| `id` | `TEXT` | O | PK | 행사 참여 ID |
| `person_id` | `TEXT` | O | FK | 참여한 사람 ID |
| `account_id` | `TEXT` | X | FK | 참여 당시 관련 관계처 ID |
| `calendar_event_id` | `TEXT` | O | 논리 FK | Calendar 행사 ID |
| `status` | `TEXT` | O | CHECK | `registered`, `attended`, `canceled`, `no_show` |
| `registered_at` | `TEXT` | X |  | 등록 시각 |
| `attended_at` | `TEXT` | X |  | 실제 참여 확인 시각 |
| `note` | `TEXT` | X |  | 참여 관련 비고 |
| 공통 컬럼 |  |  |  | 생성·수정·보관 정보 |

같은 사람과 같은 Calendar 행사 조합은 한 번만 저장한다. `account_id`가 있으면 해당 사람과 관계처의 소속 관계가 존재해야 한다.

## 외부 연동

### `crm_resource_links`

CRM 레코드 하나에 연결할 수 있는 외부 자원 개수에는 제한을 두지 않는다.

| 컬럼 | 타입 | 필수 | 키 | 저장 내용 |
|:---|:---:|:---:|:---:|:---|
| `id` | `TEXT` | O | PK | 연결 ID |
| `crm_entity_type` | `TEXT` | O | CHECK | `account`, `person`, `opportunity`, `activity`, `subscription`, `event_participation` |
| `crm_entity_id` | `TEXT` | O | 논리 FK | 연결할 CRM 레코드 ID |
| `service` | `TEXT` | O | CHECK | `flow`, `calendar`, `mail`, `files` |
| `resource_type` | `TEXT` | O |  | `task`, `event`, `message`, `file` 등 서비스 자원 종류 |
| `external_resource_id` | `TEXT` | O | 논리 FK | 외부 서비스의 실제 ID |
| `external_url` | `TEXT` | X |  | 외부 자원 바로가기 URL |
| `created_at` | `TEXT` | O |  | 연결 생성 시각 |
| `created_by_person_id` | `TEXT` | O | 논리 FK | 연결 생성 사용자 ID |
| `removed_at` | `TEXT` | X |  | 연결 해제 시각 |
| `removed_by_person_id` | `TEXT` | X | 논리 FK | 연결 해제 사용자 ID |

### `crm_integration_outbox`

| 컬럼 | 타입 | 필수 | 키 | 저장 내용 |
|:---|:---:|:---:|:---:|:---|
| `id` | `TEXT` | O | PK | 연동 작업 ID |
| `crm_entity_type` | `TEXT` | O |  | CRM 엔터티 종류 |
| `crm_entity_id` | `TEXT` | O | 논리 FK | CRM 레코드 ID |
| `target_service` | `TEXT` | O | CHECK | 대상 서비스 |
| `operation` | `TEXT` | O | CHECK | `create`, `update`, `delete` |
| `payload_json` | `TEXT` | O |  | 외부 호출 요청 데이터 |
| `idempotency_key` | `TEXT` | O | UNIQUE | 중복 실행 방지 키 |
| `status` | `TEXT` | O | CHECK | `pending`, `processing`, `succeeded`, `failed`, `canceled` |
| `attempt_count` | `INTEGER` | O | CHECK | 실행 시도 횟수 |
| `available_at` | `TEXT` | O |  | 최초 실행 또는 다음 재시도 가능 시각 |
| `lease_owner` | `TEXT` | X |  | 현재 작업을 처리 중인 워커 ID |
| `lease_expires_at` | `TEXT` | X |  | 워커 점유 만료 시각 |
| `last_error_code` | `TEXT` | X |  | 마지막 오류 코드 |
| `last_error_message` | `TEXT` | X |  | 마지막 오류 내용 |
| `result_resource_link_id` | `TEXT` | X | FK | 생성된 외부 자원 연결 ID |
| `created_at` | `TEXT` | O |  | 작업 생성 시각 |
| `updated_at` | `TEXT` | O |  | 상태 수정 시각 |
| `processed_at` | `TEXT` | X |  | 최종 처리 시각 |

워커는 처리 시작 시 lease를 획득한다. lease가 만료된 작업은 다른 워커가 다시 처리할 수 있으며 `idempotency_key`로 외부 자원의 중복 생성을 방지한다.

## CSV 가져오기

### `crm_import_jobs`

| 컬럼 | 타입 | 필수 | 키 | 저장 내용 |
|:---|:---:|:---:|:---:|:---|
| `id` | `TEXT` | O | PK | 가져오기 작업 ID |
| `source_file_name` | `TEXT` | O |  | 원본 CSV 파일명 |
| `source_file_resource_id` | `TEXT` | X | 논리 FK | Files의 원본 CSV 파일 ID |
| `column_mapping_json` | `TEXT` | O |  | 원본 컬럼과 CRM 필드 매핑 |
| `status` | `TEXT` | O | CHECK | `uploaded`, `validating`, `review`, `importing`, `completed`, `failed`, `canceled` |
| `total_rows` | `INTEGER` | O | CHECK | 전체 행 수 |
| `valid_rows` | `INTEGER` | O | CHECK | 정상 행 수 |
| `invalid_rows` | `INTEGER` | O | CHECK | 오류 행 수 |
| `imported_rows` | `INTEGER` | O | CHECK | 반영된 행 수 |
| `skipped_rows` | `INTEGER` | O | CHECK | 건너뛴 행 수 |
| `requested_by_person_id` | `TEXT` | O | 논리 FK | 실행 사용자 ID |
| `created_at` | `TEXT` | O |  | 작업 생성 시각 |
| `finished_at` | `TEXT` | X |  | 완료·실패·취소로 최종 상태가 된 시각 |
| `raw_rows_delete_after` | `TEXT` | X |  | 원본 행 삭제 예정 시각 |
| `raw_rows_deleted_at` | `TEXT` | X |  | 원본 행 실제 삭제 시각 |
| `error_summary` | `TEXT` | X |  | 작업 전체 오류 요약 |

`completed`, `failed`, `canceled` 상태가 되면 `raw_rows_delete_after`를 `finished_at`의 30일 후로 설정한다. 30일 후 원본 파일과 행별 데이터는 삭제하고 `source_file_resource_id`를 비운다. 작업 상태·건수·실행자·완료 시각은 유지한다.

### `crm_import_rows`

| 컬럼 | 타입 | 필수 | 키 | 저장 내용 |
|:---|:---:|:---:|:---:|:---|
| `job_id` | `TEXT` | O | PK, FK | 가져오기 작업 ID |
| `row_number` | `INTEGER` | O | PK | 원본 CSV 행 번호 |
| `raw_values_json` | `TEXT` | O |  | 원본 행 데이터 |
| `normalized_values_json` | `TEXT` | X |  | CRM 형식으로 변환한 데이터 |
| `validation_errors_json` | `TEXT` | X |  | 필드별 검증 오류 |
| `duplicate_person_ids_json` | `TEXT` | X |  | 이메일·전화번호로 발견한 중복 후보 사람 ID 목록 |
| `selected_person_id` | `TEXT` | X | FK | 업데이트 대상으로 선택한 기존 사람 ID |
| `selected_action` | `TEXT` | X | CHECK | `create`, `update`, `skip` |
| `status` | `TEXT` | O | CHECK | `pending`, `valid`, `invalid`, `review`, `imported`, `skipped`, `failed` |
| `error_message` | `TEXT` | X |  | 행 반영 실패 내용 |

중복 후보가 발견되면 자동 병합하지 않는다. 이메일과 전화번호가 모두 같은 사람을 가리키면 `기존 정보 업데이트`를 기본 선택으로 보여주되 사용자가 반영 전에 확인한다. 사용자는 `기존 정보 업데이트`, `새 사람으로 생성`, `건너뛰기` 중 하나를 선택할 수 있다. 이메일과 전화번호가 서로 다른 기존 사람을 가리키면 충돌로 표시하고 자동 처리하지 않는다.

### `crm_import_row_results`

CSV 한 행에서 관계처·사람·소속·진행 건 등 여러 레코드를 만들거나 수정할 수 있으므로 결과를 별도 행으로 저장한다.

| 컬럼 | 타입 | 필수 | 키 | 저장 내용 |
|:---|:---:|:---:|:---:|:---|
| `job_id` | `TEXT` | O | PK, FK | 가져오기 작업 ID |
| `row_number` | `INTEGER` | O | PK, FK | 원본 CSV 행 번호 |
| `sequence` | `INTEGER` | O | PK | 같은 원본 행 안의 처리 순서 |
| `entity_type` | `TEXT` | O | CHECK | 생성·수정된 CRM 엔터티 종류 |
| `entity_id` | `TEXT` | O | 논리 FK | 생성·수정된 CRM 레코드 ID |
| `action` | `TEXT` | O | CHECK | `created`, `updated`, `linked` |
| `created_at` | `TEXT` | O |  | 반영 시각 |

## CSV 내보내기

### `crm_export_jobs`

| 컬럼 | 타입 | 필수 | 키 | 저장 내용 |
|:---|:---:|:---:|:---:|:---|
| `id` | `TEXT` | O | PK | 내보내기 작업 ID |
| `entity_type` | `TEXT` | O | CHECK | 추출할 CRM 데이터 종류 |
| `filters_json` | `TEXT` | O |  | 검색·필터 조건 |
| `status` | `TEXT` | O | CHECK | `pending`, `processing`, `completed`, `failed`, `expired` |
| `row_count` | `INTEGER` | X | CHECK | 생성된 행 수 |
| `file_resource_id` | `TEXT` | X | 논리 FK | Files의 결과 CSV 파일 ID |
| `requested_by_person_id` | `TEXT` | O | 논리 FK | 실행 사용자 ID |
| `created_at` | `TEXT` | O |  | 요청 시각 |
| `completed_at` | `TEXT` | X |  | 파일 생성 완료 시각 |
| `file_delete_after` | `TEXT` | X |  | 결과 파일 삭제 예정 시각 |
| `file_deleted_at` | `TEXT` | X |  | 결과 파일 실제 삭제 시각 |
| `error_summary` | `TEXT` | X |  | 실패 내용 |

파일 생성이 완료되면 `file_delete_after`를 완료 시각의 7일 후로 설정한다. 7일 후 결과 파일을 삭제하고 `file_resource_id`를 비운다. 누가 언제 어떤 조건으로 내보냈는지에 대한 작업 기록은 유지한다.

## 감사 이력

### `crm_audit_events`

| 컬럼 | 타입 | 필수 | 키 | 저장 내용 |
|:---|:---:|:---:|:---:|:---|
| `id` | `TEXT` | O | PK | 감사 이벤트 ID |
| `entity_type` | `TEXT` | O |  | 변경된 CRM 엔터티 또는 작업 종류 |
| `entity_id` | `TEXT` | X | 논리 FK | 변경된 CRM 레코드 ID |
| `action` | `TEXT` | O | CHECK | `create`, `update`, `archive`, `restore`, `import`, `export`, `link`, `unlink`, `expire` |
| `changed_fields_json` | `TEXT` | X |  | 변경된 필드와 필요한 변경 전후 값 |
| `actor_type` | `TEXT` | O | CHECK | `person`, `system` |
| `actor_person_id` | `TEXT` | X | 논리 FK | 사용자 작업이면 실행자 ID |
| `actor_service` | `TEXT` | X |  | 시스템 작업이면 실행 서비스 |
| `request_id` | `TEXT` | X |  | 요청·작업 추적 ID |
| `occurred_at` | `TEXT` | O |  | 변경 시각 |
| `delete_after` | `TEXT` | O | CHECK | 감사 이벤트 삭제 가능 시각 |

감사 로그의 최소 보관 기간은 1년이며 기본 보관 기간은 3년이다. `delete_after`의 기본값은 `occurred_at`의 3년 후이며 1년보다 짧게 설정할 수 없다. CSV 원본 행, 내보낸 파일 내용, 결제 정보는 감사 로그에 복사하지 않는다.

## 권한

| 작업 | 허용 대상 |
|:---|:---|
| CRM 데이터 조회 | 모든 내부 사용자 |
| 관계처·사람·진행 건·활동·구독·행사 참여 생성·수정 | 레코드 담당자 또는 담당 팀 구성원 |
| 담당자·담당 팀 변경 | 레코드 수정 권한이 있는 사용자 |
| CSV 가져오기 | 레코드 생성·수정 권한이 있는 내부 사용자 |
| CSV 내보내기 | 관리자 |
| 레코드 보관·복원 | 관리자 |
| 감사 로그 조회 | 관리자 |

담당 팀 권한은 기존 조직도의 `circle_id`와 구성원 정보를 기준으로 판단한다. 활동·구독·행사 참여는 연결된 진행 건, 관계처 또는 사람의 담당자·담당 팀 권한을 따른다. CSV 가져오기로 기존 레코드를 수정할 때도 각 레코드의 수정 권한을 확인한다. 보관은 소프트 삭제로 처리하며 v1에서는 영구 삭제 기능을 제공하지 않는다.

## 저장하지 않거나 원본을 복제하지 않는 값

| 화면에 보이는 값 | 처리 방식 |
|:---|:---|
| 내부 담당자 이름·이메일·팀 이름 | `person_id`, `circle_id`로 기존 조직도에서 조회 |
| 마지막 접촉일 | 가장 최근 활동 시각으로 계산 |
| 다음 후속 조치일 | 연결된 Flow 업무에서 조회 |
| 진행 중인 건수 | 종료되지 않은 진행 건 개수로 계산 |
| 예상 금액 합계 | 진행 건 금액을 통화별로 합산 |
| 단계 경과 기간 | 최근 단계 변경일부터 계산 |
| 사업 표시 이름 | `business_key`로 기존 사업 정의에서 조회 |
| Flow 업무 상태·담당자·기한 | Flow가 원본으로 관리 |
| Calendar 행사 제목·시간·장소 | Calendar가 원본으로 관리 |
| Calendar 등록 상태 | outbox 상태와 외부 연결 여부로 계산 |
| 메일 본문과 첨부파일 | Mail과 Files가 원본으로 관리 |
| 결제 수단·카드 정보·청구 원문 | 결제 또는 상품 서비스가 원본으로 관리 |
| KPI와 파이프라인 집계 | CRM 원본 데이터에서 계산 |
| CSV 파일 원문 | Files가 보관 기간 동안 저장하고 CRM에는 ID만 보관 |
| Intake Draft | v1에서는 생성하거나 저장하지 않음 |

## 주요 제약조건

| 테이블 | 제약조건 |
|:---|:---|
| `crm_people` | 이름·이메일·전화번호 필수, 정규화 이메일·전화번호 인덱스 |
| `crm_account_contacts` | `(account_id, person_id)` 중복 금지, 관계처별 종료되지 않은 주 담당자는 최대 한 명 |
| `crm_opportunities` | 관계처·사람 중 하나 이상 필수, 둘 다 있으면 소속 관계 필수 |
| `crm_opportunities` | 같은 단계의 `stage_position` 인덱스, 단계 이동 시 이력 생성 |
| `crm_activities` | 관계처·사람·진행 건 중 하나 이상 필수, 진행 건 연결 값과 불일치 금지 |
| `crm_subscriptions` | `(source_service, external_subscription_id)` 중복 금지, 같은 서비스 계정·상품의 동시 활성 구독 중복 금지 |
| `crm_event_participations` | `(person_id, calendar_event_id)` 중복 금지 |
| `crm_resource_links` | 연결 개수는 무제한, 같은 CRM 레코드와 외부 자원의 활성 중복 연결 금지 |
| `crm_integration_outbox` | `idempotency_key` 유일성, 유효한 lease를 가진 워커만 상태 변경 |
| `crm_import_rows` | `(job_id, row_number)` 복합 기본키 |
| `crm_import_row_results` | `(job_id, row_number, sequence)` 복합 기본키 |
| `crm_export_jobs` | 관리자만 생성 가능, 완료 파일은 7일 후 삭제 |
| `crm_audit_events` | 최소 1년, 기본 3년 보관 |
| 주요 테이블 | enum, 금액, 날짜, 불리언 값에 `CHECK` 적용 |

## 보관 및 정리 정책

| 데이터 | 보관 기간 | 정리 후 유지하는 정보 |
|:---|:---|:---|
| CSV 가져오기 원본 파일·행별 데이터 | 작업 완료·실패·취소 후 30일 | 작업 상태, 건수, 실행자, 완료 시각 |
| CSV 내보내기 결과 파일 | 파일 생성 완료 후 7일 | 필터, 행 수, 실행자, 완료 시각 |
| 감사 로그 | 최소 1년, 기본 3년 | 보관 기간 종료 후 삭제 가능 |
| 보관된 CRM 핵심 레코드 | v1 자동 삭제 없음 | 전체 레코드와 보관 이력 |

정리 작업은 하루 한 번 실행한다. 기준 시각 이전의 데이터만 정리하며 같은 정리 작업이 반복 실행되어도 결과가 달라지지 않도록 구현한다.
