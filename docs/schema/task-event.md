# task_event

## 목적

task의 생성, 상태 변경, 담당자 변경, 일반 수정 같은 이력을 남기는 이벤트 테이블이다.

누가 언제 무엇을 바꿨는지 추적할 수 있게 한다.

## 컬럼

| 컬럼 | 타입 | 설명 |
|---|---|---|
| `id` | uuid | task_event 기본 키 |
| `task_id` | uuid | 대상 task id |
| `actor_staff_id` | uuid nullable | 이 변경을 일으킨 staff id |
| `event_type` | enum | 이벤트 종류 |
| `field_name` | text nullable | 어떤 필드가 바뀌었는지 |
| `old_value` | jsonb nullable | 이전 값 |
| `new_value` | jsonb nullable | 변경된 값 |
| `note` | text nullable | 사람이 읽는 메모 |
| `created_at` | timestamptz | 이벤트 발생 시각 |

## Enum Values

### `event_type`

- `created`
- `updated`
- `status_changed`
- `assignee_added`
- `assignee_removed`
- `paused`
- `stopped`
- `completed`
- `rejected`
- `commented`

## 메모

- 일반적인 필드 수정은 `updated`로 기록하고, `field_name`, `old_value`, `new_value`에 변경 내용을 남긴다.
- 상태 전이는 `status_changed`로 남길 수 있고, 필요하면 `paused`, `stopped`, `completed`, `rejected` 같은 의미 있는 이벤트를 별도로 남길 수도 있다.
- 시스템 자동 처리라면 `actor_staff_id`는 nullable로 둘 수 있다.
- 한 번 생성된 이벤트 row는 수정하지 않는 append-only 성격으로 운영하는 게 좋다.

## 관계

- `task_id` -> `task.id`
- `actor_staff_id` -> `staff.id`
