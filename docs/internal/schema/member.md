# member

## 목적

회사의 인원을 저장하는 기본 테이블이다.

대표를 포함한 내부 구성원을 관리하며, 다른 테이블은 이 테이블의 `id`를 외래키로 참조한다.

## 컬럼

| 컬럼 | 타입 | 설명 |
|---|---|---|
| `id` | uuid | member 기본 키 |
| `name` | text | 이름 |
| `email` | text nullable | 회사에서 사용하는 기본 이메일 |
| `title` | text nullable | 직급 또는 직책 |
| `department` | text nullable | 소속 조직 또는 팀 |
| `employment_type` | enum nullable | 고용 형태 |
| `hire_date` | date nullable | 입사일 |
| `termination_date` | date nullable | 퇴사일 |
| `status` | enum nullable | 재직 상태 |
| `created_at` | timestamptz | 생성 시각 |
| `updated_at` | timestamptz | 수정 시각 |

## Enum Values

### `employment_type`

- `full_time`
- `contract`
- `intern`
- `part_time`
- `freelancer`

### `status`

- `active`
- `leave`
- `terminated`

## 메모

- `termination_date`가 있더라도 이력 보존을 위해 row는 삭제하지 않는다.
- `title`은 직급, 직책, 역할 중 현재 운영에 맞는 값을 우선 저장한다.
- 대표도 별도 테이블로 분리하지 않고 `member` 안에서 관리한다.
- `email`, `title`, `department`, `employment_type`, `hire_date`, `termination_date`, `status`는 우선 선택 입력으로 둔다.

## 관계

- `task.assignee_member_id` -> `member.id`
- `task.requester_member_id` -> `member.id`
