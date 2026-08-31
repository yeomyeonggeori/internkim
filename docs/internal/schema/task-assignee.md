# task_assignee

## 목적

하나의 task를 누가 담당하는지 연결하는 조인 테이블이다.

한 사람이 맡는 일도 이 테이블에 1 row를 넣고, 여러 사람이 함께 맡는 일은 여러 row로 표현한다.

## 컬럼

| 컬럼 | 타입 | 설명 |
|---|---|---|
| `id` | uuid | task_assignee 기본 키 |
| `task_id` | uuid | 연결된 task id |
| `member_id` | uuid | 담당 member id |
| `role` | enum | 담당 역할 |
| `created_at` | timestamptz | 생성 시각 |
| `updated_at` | timestamptz | 수정 시각 |

## Enum Values

### `role`

- `owner`
- `collaborator`

## 메모

- 담당자가 한 명뿐이어도 `task_assignee`에 1 row를 넣는다.
- 주담당자는 `role = owner`로 둔다.
- 공동 작업자는 `role = collaborator`로 둔다.
- 필요하면 이후 `display_order`, `assigned_at`, `unassigned_at` 같은 필드를 추가할 수 있다.

## 관계

- `task_id` -> `task.id`
- `member_id` -> `member.id`
