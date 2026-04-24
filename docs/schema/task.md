# task

## 목적

업무 투두와 요청 상태를 저장하는 테이블이다.

기존 구글시트에서 쓰던 `사업/종류/내용/목표/크기/상태/시작일/종료일/주간 코드` 구조를 기준으로 정리한다.

누가 요청했는지는 `staff.id`를 직접 참조하고, 누가 담당하는지는 별도 `task_assignee` 테이블로 관리한다.

## 컬럼

| 컬럼 | 타입 | 설명 |
|---|---|---|
| `id` | uuid | task 기본 키 |
| `business` | text nullable | 어떤 사업 또는 프로젝트에 속하는지 |
| `type` | enum nullable | 업무 종류 |
| `title` | text | 할 일 제목 |
| `description` | text nullable | 상세 설명 |
| `goal` | text nullable | 목표 또는 기대 결과 |
| `size` | enum nullable | 업무 크기 |
| `status` | enum | 업무 상태 |
| `priority` | enum nullable | 우선순위 |
| `requester_staff_id` | uuid nullable | 요청자 staff id |
| `weekly_code` | text nullable | `25W03` 같은 주간 코드 |
| `due_at` | timestamptz nullable | 예정 시각 또는 마감 시각 |
| `started_at` | timestamptz nullable | 작업 시작 시각 |
| `ended_at` | timestamptz nullable | 작업 종료 시각 |
| `completed_at` | timestamptz nullable | 완료로 확정된 시각 |
| `stopped_at` | timestamptz nullable | 중단 처리 시각 |
| `rejected_at` | timestamptz nullable | 기각 처리 시각 |
| `source` | enum nullable | task가 어디서 생성되었는지 |
| `source_ref` | text nullable | 외부 문서나 시트 row 같은 원본 참조값 |
| `created_at` | timestamptz | 생성 시각 |
| `updated_at` | timestamptz | 수정 시각 |

## Enum Values

### `type`

- `general`
- `meeting`
- `document`
- `research`
- `design`
- `development`
- `operation`
- `communication`

### `size`

- `xs`
- `s`
- `m`
- `l`
- `xl`
- `xxl`

### `status`

- `requested`
- `planned`
- `paused`
- `stopped`
- `in_progress`
- `completed`
- `rejected`

### `priority`

- `low`
- `medium`
- `high`
- `urgent`

### `source`

- `manual`
- `google_sheet`
- `mattermost`
- `email`
- `system`

## 메모

- 상태값은 기존 유저 플로우의 `요청/예정/일시정지/중단/진행/완료/기각`과 기존 시트의 `상태` 컬럼에 대응한다.
- `title`은 기존 시트의 `내용` 컬럼에 대응하는 핵심 필드다.
- `goal`, `size`, `business`, `type`은 기존 시트 운영 방식을 DB로 옮기기 위한 필드다.
- 한 사람이 직접 만든 개인 할 일도 담당자는 `task_assignee`에 1 row를 넣는 방식으로 관리한다.
- 외부 요청이나 시스템 생성 task를 고려해 `requester_staff_id`는 우선 nullable로 둔다.
- 기존 시트에서 `완료` 시 시작일과 종료일을 자동 보정하던 규칙을 고려하면, DB에서도 상태 전이에 따라 `started_at`, `ended_at`, `completed_at`를 자동 관리할 수 있다.
- `weekly_code`는 `ended_at`이 있으면 종료일 기준, 없으면 `started_at` 기준으로 계산하는 파생값 성격이다.

## 관계

- `requester_staff_id` -> `staff.id`
- `task_assignee.task_id` -> `task.id`
