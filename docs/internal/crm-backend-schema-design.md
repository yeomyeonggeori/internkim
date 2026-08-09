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
| 진행 건 | `opportunity` | 영업·유치·투자·후원·제휴·조달 진행 건 |
| 진행 건 연락처 | `opportunity_contact` | 진행 건에 관여한 연락처 |
| 파이프라인 | `pipeline` | 진행 건의 성격과 돈의 방향 |
| 파이프라인 단계 | `pipeline_stage` | 파이프라인별 단계 목록 |
| 불발 사유 | `lost_reason` | 진행 건이 깨진 이유의 선택 목록 |
| 활동 | `activity` | 통화·회의·메일·메모·단계 변경 등 활동 |
| 외부 연동 | `resource_link` | Flow·Calendar·Mail·Files 연결 |

## 키 표기

| 표기 | 의미 |
|:---|:---|
| `PK` | 테이블의 각 행을 구분하는 기본키 |
| `FK` | `crm.sqlite` 내부의 다른 테이블을 참조하는 외래키 |
| `논리 FK` | 다른 서비스가 관리하는 ID를 저장하지만 DB 외래키 제약은 걸지 않는 컬럼 |
| `UNIQUE` | 같은 값의 중복 저장을 금지하는 제약 |
| `CHECK` | enum, 날짜, 시각, 금액, 연결 조건 등 허용 범위를 검증하는 제약 |
| `NOT NULL` | 값을 비울 수 없는 컬럼 |
| 부분 유니크 인덱스 | 조건을 만족하는 행에만 유일성을 요구하는 인덱스 |
| 트리거 | 다른 테이블을 함께 봐야 판단되어 `CHECK`로 쓸 수 없는 규칙 |
| 코드 | DB가 판별할 수 없어 저장 전에 애플리케이션이 확인하는 규칙 |

## 날짜와 시각 형식

시각 값은 SQLite에 `TEXT`로 저장하고 UTC RFC 3339 형식으로 통일한다.

| 형식 | 예시 | 사용하는 컬럼 |
|:---|:---|:---|
| `YYYY-MM-DDTHH:MM:SSZ` | `2026-07-27T09:15:00Z` | 이름이 `_at`으로 끝나는 시각 컬럼 |

- 실제 시각은 항상 UTC로 저장한다. `Z` 이외의 오프셋과 소수점 이하 초는 저장하지 않는다.
- 시각 문자열의 자릿수를 고정해 문자열 정렬이 시간 순서와 같도록 한다.
- 사용자 표시 시각의 시간대 변환은 클라이언트가 처리한다.
- 날짜만 입력한 마감은 클라이언트가 사용자 시간대의 그날 `23:59:59`를 UTC로 변환해 `due_at`에 저장하고, 그때 고른 시간대를 `due_time_zone`에 남긴다.
- 저장 값에 `CHECK`를 걸어 형식을 강제한다.

```sql
CHECK (created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9]Z')
```

시각 형식은 SQLite의 `datetime()`, `julianday()`, `strftime()`이 그대로 인식한다.

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
| `status` | `TEXT` | O | CHECK | `prospect`, `active`, `paused` |
| `types` | `TEXT` | X |  | `customer`, `partner`, `sponsor`, `vendor`, `investor`, `portfolio`, `other` 중 복수 선택 |
| `tags` | `TEXT` | O |  | 검색·분류용 태그 배열 |
| `importance` | `TEXT` | O | CHECK | `high`, `medium`, `low` |
| `owner_person_id` | `TEXT` | O | 논리 FK | 내부 담당자 ID |
| `owner_circle_id` | `TEXT` | X | 논리 FK | 담당 팀·조직 ID |
| `address` | `TEXT` | X |  | 주소 |
| `description` | `TEXT` | X |  | 비고와 설명 |
| 공통 컬럼 |  |  |  | 생성·수정·보관 정보 |

`types`는 비워둘 수 있다. `status = 'prospect'`인 관계처는 아직 어떤 상대가 될지 모르는 상태라, 유형을 강제하면 `other`나 짐작한 값이 들어가고 그 값은 나중에 고쳐지지 않는다.

`status`와 `importance`는 서로 다른 축이다. `status`는 거래가 어느 단계인지를, `importance`는 그 관계처를 얼마나 유망하게 보는지를 나타낸다. 두 축을 한 목록에 섞으면 `paused`이면서 유망한 관계처를 표현할 수 없다. 관심 온도를 뜻하던 `warm`, `cold`는 `importance`의 `high`, `low`와 같은 축이라 `status`에서 뺐고, 아직 거래를 시작하지 않은 상태는 `prospect`로 부른다.

`closed`도 뺐다. 진행 건 하나가 깨진 것은 `opportunity`의 `outcome`이 `lost`인 것이지 관계처의 상태가 아니다. 한 관계처에 건이 여러 개인데 그중 하나가 불발됐다고 회사 전체를 종료로 표시할 수 없다. 관계 자체가 끝난 경우는 `archived_at`이 이미 담당한다.

`importance`는 `task.priority`와 이름을 맞추지 않는다. `task.priority`는 `urgent`를 최상위로 갖는 시간 압박이고 이쪽은 얼마나 중요한 상대인지를 나타내는 다른 축이다. 진행 건의 시간 압박은 `due_at`이 이미 표현한다. 이름만 같고 값이 다르면 나중에 상수나 타입을 공유하려다 `urgent`가 흘러든다.

## 연락처

### `contact`

B2B 담당자와 B2C 개인 고객을 함께 저장한다. 소속이 없는 개인 고객은 `account_id`를 비운다.

v1은 한 사람이 하나의 관계처에 소속된다고 본다. 소속 정보를 별도 조인 테이블로 두지 않고 이 테이블에 직접 저장한다.

| 컬럼 | 타입 | 필수 | 키 | 저장 내용 |
|:---|:---:|:---:|:---:|:---|
| `id` | `TEXT` | O | PK | 연락처 ID |
| `account_id` | `TEXT` | X | FK | 소속 관계처 ID |
| `name` | `TEXT` | O |  | 이름 |
| `email` | `TEXT` | X |  | 이메일 |
| `phone` | `TEXT` | X |  | 전화번호 |
| `title` | `TEXT` | X |  | 직함·역할 |
| `department` | `TEXT` | X |  | 부서 |
| `is_primary` | `INTEGER` | O | CHECK | 소속 관계처의 주 담당자 여부 |
| `owner_person_id` | `TEXT` | O | 논리 FK | 내부 담당자 ID |
| `owner_circle_id` | `TEXT` | X | 논리 FK | 담당 팀·조직 ID |
| `description` | `TEXT` | X |  | 비고 |
| 공통 컬럼 |  |  |  | 생성·수정·보관 정보 |

이메일과 전화번호는 둘 중 하나 이상 있어야 한다. 둘 다 강제하면 명함에 한쪽만 있는 사람에게 `010-0000-0000` 같은 값이 들어가고, 그 값이 아래 중복 검색 인덱스에 걸려 서로 무관한 사람들이 중복 후보로 뜬다.

중복 검색용 정규화 컬럼을 따로 두지 않고 표현식 인덱스로 처리한다.

```sql
CREATE INDEX contact_email_lookup ON contact (lower(trim(email)));
CREATE INDEX contact_phone_lookup ON contact (replace(replace(replace(phone, '-', ''), ' ', ''), '+82', '0'));
```

같은 값이 발견되어도 자동 병합하지 않고 사용자 검토 대상으로 표시한다. Salesforce, HubSpot, Odoo 모두 자동으로 합치지 않고 어느 쪽을 살릴지 사용자가 고르게 한다.

v1은 검토까지만 한다. 중복으로 확인되면 한쪽을 보관하고, 필요하면 활동 몇 개를 손으로 옮긴다. 가져오기 흐름이 행마다 `기존 정보 업데이트`를 고르게 해서 가장 큰 유입구를 이미 막고 있어, 남는 중복은 두 사람이 같은 사람을 각각 만드는 드문 경우뿐이다. 그 정도는 손으로 처리하는 편이 네 테이블을 재부모화하는 병합 기능보다 싸다.

## 진행 건

### `opportunity`

진행 건은 관계처, 연락처 또는 둘 모두에 연결할 수 있다. 연락처는 `opportunity_contact`로 여러 명 붙일 수 있다.

`account.types`는 관계처가 우리에게 어떤 상대인지를, `opportunity.pipeline`은 개별 건의 성격을 나타낸다. 아래처럼 대응하지만 서로 자동으로 갱신하지 않는다. 고객사가 동시에 투자자일 수 있고, 거래가 모두 끝난 뒤에도 관계처의 성격은 남기 때문이다. 진행 건을 만들 때 대응하는 유형을 화면에서 제안할 수는 있으나 저장은 사용자가 확정한다.

| `account.types` | `opportunity.pipeline` |
|:---|:---|
| `customer` | `sales` |
| `investor` | `fundraising` |
| `portfolio` | `investment` |
| `sponsor` | `sponsorship` |
| `partner` | `partnership` |
| `vendor` | `procurement` |

`other`에 대응하는 파이프라인은 없다. 투자는 방향에 따라 상대가 다르다. 우리가 투자를 받으면 상대가 `investor`이고 파이프라인은 `fundraising`, 우리가 투자하면 상대가 `portfolio`이고 파이프라인은 `investment`다. 파이프라인을 새로 추가할 때 대응하는 `types` 값이 필요하면 함께 넣는다. `pipeline`은 `pipeline` 테이블의 행이지만 `types`는 JSON 배열 컬럼이라 배열 원소에 외래키를 걸 수 없어, 이 대응은 DB가 강제하지 못하고 사람이 맞춘다.

| 컬럼 | 타입 | 필수 | 키 | 저장 내용 |
|:---|:---:|:---:|:---:|:---|
| `id` | `TEXT` | O | PK | 진행 건 ID |
| `account_id` | `TEXT` | X | FK | 관련 관계처 ID |
| `business` | `TEXT` | X | 논리 FK | 어떤 사업 또는 프로젝트에 속하는지 |
| `name` | `TEXT` | O |  | 진행 건 이름 |
| `pipeline` | `TEXT` | O | FK | 진행 건의 성격이자 파이프라인 |
| `stage` | `TEXT` | O | FK | 현재 파이프라인 단계 |
| `stage_position` | `REAL` | O |  | 같은 단계 내 수동 카드 정렬 순서 |
| `stage_changed_at` | `TEXT` | O | CHECK | 현재 단계로 이동한 시각 |
| `owner_person_id` | `TEXT` | O | 논리 FK | 내부 담당자 ID |
| `owner_circle_id` | `TEXT` | X | 논리 FK | 담당 팀·조직 ID |
| `amount_minor` | `INTEGER` | X | CHECK | 금액의 최소 화폐 단위 |
| `currency_code` | `TEXT` | O | CHECK | `KRW`, `USD`, `JPY`, `EUR` |
| `base_amount_minor` | `INTEGER` | X | CHECK | 기준 통화로 환산한 금액 |
| `base_currency_code` | `TEXT` | X | CHECK | 환산에 쓴 기준 통화 |
| `importance` | `TEXT` | O | CHECK | `high`, `medium`, `low` |
| `due_at` | `TEXT` | X | CHECK | UTC 기준 마감 시각 |
| `due_time_zone` | `TEXT` | X |  | 마감을 입력한 IANA 시간대 |
| `lost_reason` | `TEXT` | X | FK | 불발 사유 |
| `description` | `TEXT` | X |  | 진행 건 설명 |
| 공통 컬럼 |  |  |  | 생성·수정·보관 정보 |

`(pipeline, stage)`는 `pipeline_stage`를 참조한다. 진행 건은 항상 하나의 파이프라인에만 속하고, 그 파이프라인에 정의되지 않은 단계로는 이동할 수 없다.

진행 건의 성격과 파이프라인을 따로 두지 않는다. 어떤 성격의 건인지가 곧 어떤 단계를 밟는지라서 값 하나로 둘 다 정해진다. HubSpot과 Pipedrive도 같은 이유로 별도 종류 필드 없이 파이프라인 참조 하나만 둔다.

필수 컬럼이 많지만 사용자가 입력해야 하는 값은 `name`, `pipeline`, `owner_person_id` 셋뿐이다. 파이프라인 첫 단계에 들어오는 건은 아직 통화도 담당 팀도 정해지지 않은 문의라, 입력할 것이 많으면 아예 기록되지 않는다. 나머지는 저장 시점에 채운다.

| 컬럼 | 채우는 값 |
|:---|:---|
| `stage` | 해당 파이프라인에서 `position`이 가장 작은 단계 |
| `stage_position` | 그 단계 맨 앞 카드 앞의 값 |
| `stage_changed_at` | `created_at`과 같은 값 |
| `currency_code` | 워크스페이스 기본 통화 |
| `importance` | `medium` |

`currency_code`는 화면에서 묻지 않고 기본 통화를 넣는다. 금액을 적을 때만 통화 선택을 보여준다. 통화를 레코드 단위로 항상 채워두는 건 금액 필드가 나중에 늘어나도 한 통화가 전부를 지배하게 하려는 것이지, 사용자에게 고르게 하려는 것이 아니다.

`lost_reason`은 `outcome`이 `lost`인 단계에서만 채운다. 다른 단계로 되돌리면 비운다.

`base_amount_minor`와 `base_currency_code`는 진행 중인 건에서는 비워둔다. 아직 실현되지 않은 금액이라 오늘 환율로 보는 것이 맞고, 통화가 섞인 파이프라인 합계는 클라이언트가 실시간 환율로 환산한다.

`outcome`이 `won`이나 `lost`가 되는 시점에 그때 환율로 환산해 두 값을 채우고 이후로는 건드리지 않는다. 그래야 지난 분기 실적이 오늘 환율에 따라 다시 계산되지 않는다. 미실현은 오늘 환율, 실현은 그때 환율이다.

환율 자체는 저장하지 않는다. 환율을 두면 합계마다 실수 곱셈이 들어가 반올림이 질의마다 미세하게 달라지는데, 환산액을 정수로 박아두면 합계가 그냥 `SUM`이다. `currency_code`가 기준 통화와 같으면 환산 없이 `amount_minor`를 복사한다. 성사된 건의 금액을 나중에 고치면 `base_amount_minor`도 기존 두 값의 비율로 다시 계산한다.

단계를 옮기면 `stage_changed_at`을 갱신하고 `activity`에 `kind = 'stage_change'` 행을 함께 만든다. 두 가지가 한 트랜잭션 안에서 일어나야 단계 경과 기간과 변경 이력이 어긋나지 않는다.

`stage_position`은 실수로 저장하고 카드를 옮기면 앞뒤 카드 값의 중간값을 넣는다. 계산한 중간값이 앞뒤 값과 같아지거나 동일한 정렬값이 충돌하면 해당 단계의 카드만 `1024.0`, `2048.0`, `3072.0`처럼 일정 간격으로 다시 배정한다. 재배정은 하나의 트랜잭션으로 처리하며 다른 단계의 정렬값은 변경하지 않는다.

마감이 없으면 `due_at`과 `due_time_zone`을 모두 비운다. 마감이 있으면 두 값을 모두 저장한다.

- 마감 시각 자체는 `due_at` 하나로 정한다. 입력 정밀도는 별도로 저장하지 않는다.
- 클라이언트는 날짜만 입력하면 사용자 시간대의 해당 날짜 `23:59:59`를, 시간까지 입력하면 입력한 현지 시각을 UTC로 변환해 보낸다.
- `due_time_zone`은 `Asia/Seoul`, `America/New_York` 같은 유효한 IANA 시간대만 허용한다.
- 시간대 기본값은 브라우저 시간대이며 사용자가 직접 변경할 수 있다. 변경하면 화면의 현지 날짜·시간을 유지한 채 클라이언트가 `due_at`을 다시 계산한다.
- 서머타임 전환으로 존재하지 않거나 중복되는 현지 시각은 자동 보정하지 않고 사용자가 다시 선택하게 한다.
- 알림, 기한 초과 판정, 시간순 정렬, 월·분기 리포트는 모두 `due_at`을 기준으로 계산한다.
- 날짜 전용 마감의 표시는 `due_at`을 `due_time_zone`으로 변환해 판별한다. 변환한 현지 시각이 `23:59:59`면 시각을 감추고 날짜만 보여준다. 별도의 정밀도 컬럼을 두지 않는 이유다.

단계 변경 이력은 `activity`에 `kind = 'stage_change'` 로 기록한다. 별도 이력 테이블을 두지 않는다.

### `opportunity_contact`

규모 있는 건에는 결정권자와 실무자가 따로 있다. 진행 건에 연락처를 하나만 붙이면 나머지는 활동 본문에 이름으로만 남아서 "이 건에 누가 관여했나"를 물을 수 없다.

`opportunity`에 `contact_id`를 남겨두고 이 테이블을 함께 두면 주 연락처의 진실이 두 군데가 되므로, 연락처 연결은 전부 이 테이블로 옮긴다.

| 컬럼 | 타입 | 필수 | 키 | 저장 내용 |
|:---|:---:|:---:|:---:|:---|
| `opportunity_id` | `TEXT` | O | PK, FK | 진행 건 ID |
| `contact_id` | `TEXT` | O | PK, FK | 연락처 ID |
| `is_primary` | `INTEGER` | O | CHECK | 이 건의 주 연락처 여부 |

기본키가 `(opportunity_id, contact_id)`라 같은 사람을 두 번 붙일 수 없다. 진행 건마다 `is_primary`인 행은 최대 하나이며 `contact.is_primary`와 같은 방식이다.

역할 컬럼은 두지 않는다. v1에서 필요한 것은 "이 건으로 누구에게 연락하는가"이고 `is_primary`가 그 답이다. "결정권자가 누구였길래 깨졌는가"를 묻기 시작하면 그때 이 테이블에 컬럼 하나를 늘린다.

### `pipeline`

진행 건의 성격이자 단계 목록의 주인이다. 파이프라인마다 돈이 흐르는 방향이 달라 이 값을 여기에 둔다.

| 컬럼 | 타입 | 필수 | 키 | 저장 내용 |
|:---|:---:|:---:|:---:|:---|
| `pipeline` | `TEXT` | O | PK | 파이프라인 값 |
| `label` | `TEXT` | O |  | 화면에 보여줄 이름 |
| `direction` | `TEXT` | O | CHECK | `inbound`, `outbound`, `none` |
| `is_active` | `INTEGER` | O | CHECK | 새 진행 건에서 고를 수 있는지 여부 |

`direction`은 성사된 금액이 들어오는지 나가는지를 정한다. 이게 없으면 조달 계약 금액이 매출 합계에 더해진다. 사무실 집기를 산 건과 제품을 판 건이 같은 숫자로 들어가는 것이다.

`none`은 돈이 오가지 않는 파이프라인이다. 제휴가 여기 해당한다. 금액을 적더라도 어느 쪽 합계에도 넣지 않는다. 제휴에서 실제로 수익이 발생하면 그것은 별도의 `sales` 진행 건으로 기록한다. 제휴 자체의 금액은 기대값이지 오간 돈이 아니다.

v1은 여섯으로 시작한다.

| `pipeline` | `label` | `direction` |
|:---|:---|:---|
| `sales` | 영업 | `inbound` |
| `fundraising` | 투자 유치 | `inbound` |
| `sponsorship` | 후원 | `inbound` |
| `investment` | 투자 집행 | `outbound` |
| `procurement` | 조달 | `outbound` |
| `partnership` | 제휴 | `none` |

투자는 방향에 따라 파이프라인이 둘이다. 받는 쪽과 하는 쪽은 상대도 다르고 단계도 다르고 돈의 방향도 반대라 한 파이프라인에 담을 수 없다. 이름은 업계 용어를 그대로 쓴다. 유치하는 쪽이 `fundraising`, 집행하는 쪽이 `investment`다. `investment_in`처럼 방향을 접미사로 붙이지 않는다.

쓰지 않기로 한 파이프라인은 지우지 않고 `is_active`를 `0`으로 내린다. 이미 그 파이프라인으로 기록된 진행 건이 있으면 외래키가 삭제를 막는다.

### `pipeline_stage`

파이프라인마다 단계 목록이 다르다. 후원 제안과 조달은 영업과 같은 순서로 움직이지 않는다. 단계 목록을 문장이나 `CHECK`에 적어두면 한 단계를 늘릴 때마다 테이블을 다시 만들어야 하므로 데이터로 저장한다.

이 테이블은 사용자 레코드가 아니라 설정값이므로 공통 컬럼을 두지 않는다.

| 컬럼 | 타입 | 필수 | 키 | 저장 내용 |
|:---|:---:|:---:|:---:|:---|
| `pipeline` | `TEXT` | O | PK, FK | `pipeline`의 파이프라인 값 |
| `stage` | `TEXT` | O | PK | 단계 이름 |
| `position` | `INTEGER` | O |  | 보드에서 단계를 늘어놓는 순서 |
| `outcome` | `TEXT` | O | CHECK | `open`, `on_hold`, `won`, `lost` |

기본키는 `(pipeline, stage)`이고 `opportunity`가 같은 쌍으로 참조한다. 두 컬럼을 따로 `CHECK`하는 대신 조합을 외래키로 묶어서 `pipeline = 'procurement'`인데 `stage = 'qualified'`처럼 그 파이프라인에 없는 단계를 저장할 수 없게 한다.

`position`은 칸반 보드의 열 순서다. 단계 순서를 문서 문장이 아니라 데이터로 두면 화면이 읽어서 그대로 그린다.

`outcome`은 그 단계가 어떤 결말인지를 정한다. 단계 이름은 파이프라인마다 다를 수 있지만 결말은 네 가지뿐이라, 이 값으로 파이프라인을 가리지 않고 집계할 수 있다.

`won`, `lost`가 영업 어휘로 보이지만 그대로 쓴다. NPSP는 보조금을 받는 건에도 Closed Won과 Closed Lost를 쓴다. 돈이 어느 방향으로 흐르든 업계가 같은 단어를 쓰므로 `succeeded`, `failed` 같은 말로 바꾸면 그쪽이 오히려 낯선 이름이 된다. 조달에서 성사가 곧 계약 체결이라는 것은 화면 표기로 풀 문제다.

Salesforce는 이 자리를 `IsClosed`와 `IsWon` 두 불리언으로 둔다. 우리가 값 하나로 두고 `on_hold`를 더한 것은 진행 중인 건수에서 멈춘 건을 빼기 위해서다. 불리언 둘로는 그 구분이 안 되어 결국 화면이 단계 이름을 알아야 하는데, 그러면 `outcome`을 만든 이유가 사라진다. "진행 중인 건수"는 `open`, 성사 금액은 `won`, 불발 사유는 `lost`인 단계에서만 계산한다. 종료 단계 이름을 코드에 박지 않는다.

단계 목록은 진행 방식이 실제로 다른 파이프라인에만 따로 준다. 같은 방식으로 움직이는 파이프라인은 같은 단계를 쓴다. Odoo도 단계에 붙는 팀을 비워두면 모든 파이프라인이 그 단계를 공유하는 방식으로 같은 문제를 푼다. 다만 우리는 `(pipeline, stage)` 외래키로 무결성을 걸었으므로 공유는 개념으로만 두고 행은 파이프라인마다 실제로 채운다. 설정 테이블의 행 몇 개보다 "그 파이프라인에 없는 단계로 갈 수 없다"는 보장이 값지다.

`sales`, `sponsorship`, `partnership`은 같은 단계를 쓴다.

| `position` | `stage` | `outcome` |
|:---|:---|:---|
| 1 | `lead` | `open` |
| 2 | `qualified` | `open` |
| 3 | `proposal` | `open` |
| 4 | `negotiation` | `open` |
| 5 | `won` | `won` |
| 6 | `lost` | `lost` |
| 7 | `on_hold` | `on_hold` |

파이프라인에 고유한 단계를 주는 기준은 게이트의 개수나 의미가 다른가이다. 단어만 다르면 공유한다. 같은 흐름에 다른 이름을 붙이려고 행을 늘리면 관리할 설정만 늘고 표현할 수 있는 것은 그대로다.

후원과 제휴는 고유한 단계 목록을 두지 않는다. 후원 유치 전용 파이프라인을 정의한 업계 표준이나 CRM 템플릿을 찾지 못했고, 실무에서도 영업 파이프라인을 그대로 쓴다. 제휴는 `identified`, `engaged`, `qualification`, `negotiation`, `signed`라는 업계 용어가 있지만 게이트의 수와 의미가 영업과 같아서 단어만 다르다.

조달은 다르다. `lead`와 `qualified`는 들어온 관심을 거르는 단계인데 조달의 공급사는 우리가 골라서 부른 곳이라 거를 대상이 아니다. 대응물이 없어서 고유 단계를 준다.

`fundraising`은 VC 딜플로우와 NPSP의 보조금 단계가 같은 관문을 각각 주는 쪽과 받는 쪽에서 본 것이라, 둘이 겹치는 관문을 유치하는 쪽 이름으로 옮긴 것이다. 접촉과 피칭 뒤에 실사가 오고, 상대 내부 심의를 통과해야 텀시트가 나오고, 그다음 집행이다.

| `position` | `stage` | `outcome` |
|:---|:---|:---|
| 1 | `contacted` | `open` |
| 2 | `pitched` | `open` |
| 3 | `due_diligence` | `open` |
| 4 | `committee` | `open` |
| 5 | `term_sheet` | `open` |
| 6 | `closed` | `won` |
| 7 | `lost` | `lost` |
| 8 | `on_hold` | `on_hold` |

`committee`는 상대가 진행하는 절차라 우리가 움직일 수 없지만, 여기서 막히는 기간이 길고 그 자체가 관리할 정보라 단계로 둔다. VC 표준의 investment committee, NPSP의 Under Review에 해당한다. 단계 이름에 파이프라인 이름을 접두사로 붙이지 않는다. 기본키가 `(pipeline, stage)`라 이미 스코프가 잡혀 있고, 영업 단계도 `sales_lead`가 아니라 `lead`다. 심의 절차가 없는 상대와만 일한다면 이 행만 지우면 된다.

`investment`은 VC 딜플로우 표준을 그대로 쓴다. 우리가 투자하는 쪽이므로 발굴과 검토가 앞에 붙고, 내부 심의와 텀시트를 거쳐 집행으로 끝난다. `partner_review`가 가장 좁은 관문이다.

| `position` | `stage` | `outcome` |
|:---|:---|:---|
| 1 | `sourcing` | `open` |
| 2 | `screening` | `open` |
| 3 | `partner_review` | `open` |
| 4 | `due_diligence` | `open` |
| 5 | `committee` | `open` |
| 6 | `term_sheet` | `open` |
| 7 | `closed` | `won` |
| 8 | `lost` | `lost` |
| 9 | `on_hold` | `on_hold` |

`fundraising`과 `investment`이 `due_diligence`, `committee`, `term_sheet`, `closed`를 같은 이름으로 쓴다. 같은 관문을 양쪽에서 보는 것이라 이름이 같은 게 맞고, 기본키가 `(pipeline, stage)`라 충돌하지 않는다.

`procurement`이 CRM에 있는 것은 이 문서의 판단이다. Salesforce, HubSpot, Pipedrive, Odoo, EspoCRM의 진행 건은 모두 판매 전용이고, ERPNext는 구매를 별도 모듈로 두어 Material Request, RFQ, Supplier Quotation, Purchase Order, Purchase Receipt, Purchase Invoice라는 자체 문서 체인을 갖는다. 여기서 조달은 그 체인이 아니라 "누구와 무엇을 논의 중이고 어디까지 왔나"만 추적한다. 발주서와 입고, 세금계산서가 필요해지면 조달은 CRM을 떠나 구매 시스템으로 가야 한다.

투자는 양쪽 다 CRM에 있는 것이 정상이다. 유치는 CRM 말고 기록할 데가 없다. 상대가 관계처이고 여러 번 만나고 단계가 있고 성사와 불발이 있는, CRM이 정확히 다루는 모양이다. Attio는 리스트의 용도로 영업, 채용, 고객 관리와 나란히 투자 유치를 명시한다. 집행 쪽은 투자사에게 딜플로우가 곧 CRM이라 Affinity와 Attio가 그 시장에 팔린다.

Salesforce 표준 진행 건에 유치가 없는 것은 그 모델이 B2B 영업 전용이기 때문이지 유치를 CRM에 두지 않기 때문이 아니다. 넣는 방법이 레코드 타입이나 별도 파이프라인을 만드는 것이고 이 문서가 그렇게 했다.

이 스키마는 스타트업과 투자사 양쪽이 쓸 수 있어야 한다. 스타트업은 `fundraising`으로 투자자를 관리하고, 투자사는 `investment`으로 포트폴리오 후보를 관리하면서 자기 펀드의 출자자 모집에는 다시 `fundraising`을 쓴다. `direction`은 조달을 빼더라도 이 둘 때문에 필요하다.

`procurement`은 CIPS의 sourcing process와 source-to-contract를 따른다. `rfx`는 RFI·RFP·RFQ를 아우르는 표준 용어이며 건마다 무엇을 쓸지 다르므로 단계를 셋으로 쪼개지 않는다.

| `position` | `stage` | `outcome` |
|:---|:---|:---|
| 1 | `rfx` | `open` |
| 2 | `evaluation` | `open` |
| 3 | `negotiation` | `open` |
| 4 | `contract_award` | `won` |
| 5 | `lost` | `lost` |
| 6 | `on_hold` | `on_hold` |

조달 진행 건은 공급사 하나를 가리킨다. 세 곳에 견적을 받으면 진행 건이 셋이고 하나가 `contract_award`, 둘이 `lost`에 `lost_reason`은 `competitor`가 된다. 여러 견적을 나란히 비교하는 것은 단계가 아니라 같은 사업의 진행 건을 모아 보는 화면이다.

CIPS의 sourcing process에는 앞에 요건 정의와 공급사 발굴이 더 있지만 넣지 않는다. 그 둘은 공급사가 정해지기 전에 한 번 하는 일이라 공급사 단위 레코드에 붙을 수 없다. 요건과 발굴을 관리해야 하면 진행 건들의 부모가 되는 구매 요청 레코드가 필요하고, 그건 CRM이 아니라 구매 시스템의 몫이다.

### `lost_reason`

불발 사유는 값 목록을 미리 정하지 않는다. HubSpot은 이 필드를 자유 텍스트로, Pipedrive는 자유 입력을 기본으로 두고 회사가 목록을 직접 정의하게 한다. 둘 다 미리 채워진 표준 값이 없다. 업계가 정한 목록이 없으니 우리가 정해야 하는데, `CHECK`에 박으면 값 하나 늘릴 때마다 테이블을 다시 만들어야 하고 자유 텍스트로 두면 집계가 안 된다. 그래서 `pipeline_stage`와 같이 데이터로 둔다.

| 컬럼 | 타입 | 필수 | 키 | 저장 내용 |
|:---|:---:|:---:|:---:|:---|
| `reason` | `TEXT` | O | PK | 사유 값 |
| `label` | `TEXT` | O |  | 화면에 보여줄 이름 |
| `is_active` | `INTEGER` | O | CHECK | 새 진행 건에서 고를 수 있는지 여부 |

`opportunity.lost_reason`이 `reason`을 참조한다. 순서 컬럼은 두지 않고 `label` 순으로 보여준다. 단계와 달리 사유는 나열 순서가 의미를 갖지 않으며 Odoo의 `crm.lost.reason`도 `name`과 `active` 둘뿐이다. 쓰지 않기로 한 사유는 지우지 않고 `is_active`를 `0`으로 내린다. 지우면 그 사유로 기록된 과거 건의 참조가 깨져서 지난 분기 분석이 사라진다.

시작 값은 아래로 둔다. 실제 사유가 쌓이면 고치는 게 전제다. 데이터라서 행만 바꾸면 된다.

| `reason` | `label` |
|:---|:---|
| `price` | 가격 |
| `competitor` | 경쟁사 선정 |
| `timing` | 시기 부적합 |
| `no_budget` | 예산 부재 |
| `no_fit` | 요건 불일치 |
| `no_response` | 응답 없음 |
| `internal` | 내부 사정 |

## 활동

### `activity`

| 컬럼 | 타입 | 필수 | 키 | 저장 내용 |
|:---|:---:|:---:|:---:|:---|
| `id` | `TEXT` | O | PK | 활동 ID |
| `account_id` | `TEXT` | X | FK | 관련 관계처 ID |
| `contact_id` | `TEXT` | X | FK | 관련 연락처 ID |
| `opportunity_id` | `TEXT` | X | FK | 관련 진행 건 ID |
| `business` | `TEXT` | X | 논리 FK | 어떤 사업 또는 프로젝트에 속하는지 |
| `kind` | `TEXT` | O | CHECK | `note`, `email`, `meeting`, `call`, `task`, `file`, `event`, `stage_change` |
| `title` | `TEXT` | O |  | 활동 제목 |
| `occurred_at` | `TEXT` | O | CHECK | 활동 발생 시각 |
| `content` | `TEXT` | X |  | 활동 내용 |
| 공통 컬럼 |  |  |  | 생성·수정·보관 정보 |

활동의 자유 텍스트만 `description`이 아니라 `content`다. 다른 테이블의 `description`은 구조화된 필드가 정체를 정한 뒤에 덧붙이는 부가 설명이라 비워도 레코드가 성립하지만, 활동은 이 텍스트 자체가 기록의 실체다. 비우면 시각과 종류만 남아서 남길 이유가 없어진다.

`summary`를 쓰지 않는 이유는 이 컬럼에 두 종류가 들어오기 때문이다. `kind = 'note'`인 메모는 그 자체가 원본이라 무언가의 요약이 아니고, `kind = 'email'`이나 `'file'`은 본문을 복제하지 않으므로 요지만 남는다. `summary`는 후자에만 맞아서 메모까지 그렇게 부르면 원본이 따로 있는 것처럼 읽힌다. `note`는 `kind = 'note'`와 충돌한다.

활동은 관계처·연락처·진행 건 중 하나 이상에 연결해야 한다. 진행 건에 연결하면 `account_id`와 `business`는 진행 건의 값을 따르고, `contact_id`는 그 진행 건에 연결된 연락처 중 하나여야 한다. 진행 건에 연결되지 않은 활동은 사용자가 사업을 직접 선택한다.

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
| `external_resource_type` | `TEXT` | O |  | `task`, `event`, `message`, `file` 등 서비스 자원 종류 |
| `external_resource_id` | `TEXT` | O | 논리 FK | 외부 서비스의 실제 ID |
| `external_resource_url` | `TEXT` | X |  | 외부 자원 바로가기 URL |
| `created_at` | `TEXT` | O | CHECK | 연결 생성 시각 |
| `created_by_person_id` | `TEXT` | O | 논리 FK | 연결 생성 사용자 ID |
| `removed_at` | `TEXT` | X | CHECK | 연결 해제 시각 |
| `removed_by_person_id` | `TEXT` | X | 논리 FK | 연결 해제 사용자 ID |

CRM 레코드를 먼저 저장한 뒤 외부 자원 생성을 동기로 호출한다. 성공하면 `resource_link`를 저장하고, 실패하면 CRM 레코드는 유지한 채 사용자에게 오류와 수동 재시도를 제공한다. v1에 outbox 큐, 워커 lease, 재시도 스케줄러는 두지 않는다.

## CSV 가져오기와 내보내기

가져오기는 업로드 → 미리보기 → 반영 세 단계로 처리하며 진행 상태를 DB에 저장하지 않는다.

CSV 파일은 최대 5 MB까지 허용하고 행 수는 제한하지 않는다. 5 MB를 초과하면 파싱 전에 거부한다.

1. 업로드한 CSV를 파싱해 검증 결과, 중복 후보, 컬럼 매핑을 응답으로 돌려준다.
2. 사용자가 행별로 `기존 정보 업데이트`, `새 연락처로 생성`, `건너뛰기` 중 하나를 고른다. 이메일과 전화번호가 모두 같은 기존 연락처를 가리키면 `기존 정보 업데이트`를 기본 선택으로 보여준다. 이메일과 전화번호가 서로 다른 기존 연락처를 가리키면 충돌로 표시하고 자동 처리하지 않는다.
3. 확정된 내용을 한 트랜잭션으로 반영한다. 한 행이라도 반영에 실패하면 전체를 롤백하며 부분 저장하지 않는다.

내보내기는 `GET /crm/export.csv?entity=...&filters=...` 로 행 수와 파일 크기 제한 없이 결과를 바로 스트리밍한다. 파일을 Files에 저장하거나 만료 시각을 관리하지 않는다.

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
| 마지막 접촉일 | `stage_change`를 제외한 가장 최근 활동 시각으로 계산 |
| 다음 후속 조치일 | 연결된 Flow 업무에서 조회 |
| 진행 중인 건수 | `outcome`이 `open`인 단계의 진행 건 개수로 계산 |
| 예상 금액 합계 | `outcome`이 `open`인 진행 건 금액을 `direction`과 통화별로 합산하고 환산은 클라이언트가 처리 |
| 성사 금액 합계 | `outcome`이 `won`인 진행 건의 `base_amount_minor`를 `direction`별로 합산 |
| 불발 사유별 건수 | `lost_reason`으로 집계 |
| 순 유입 금액 | `inbound` 합계에서 `outbound` 합계를 뺀 값 |
| 단계 경과 기간 | `stage_changed_at`부터 계산 |
| 사업 표시 이름 | `business`로 기존 사업 정의에서 조회 |
| Flow 업무 상태·담당자·기한 | Flow가 원본으로 관리 |
| Calendar 행사 제목·시간·장소 | Calendar가 원본으로 관리 |
| 메일 본문과 첨부파일 | Mail과 Files가 원본으로 관리 |
| 구독과 결제 정보 | 상품·결제 서비스가 원본으로 관리하며 CRM은 복제하지 않음 |
| KPI와 파이프라인 집계 | CRM 원본 데이터에서 계산 |
| CSV 원본과 내보내기 결과 파일 | CRM DB에 보관하지 않음 |
| Intake Draft | v1에서는 생성하거나 저장하지 않음 |

## 주요 제약조건

`적용` 열은 그 규칙을 무엇이 막는지 나타낸다. SQLite의 `CHECK`는 다른 테이블을 참조할 수 없으므로, 두 테이블을 함께 봐야 판단되는 규칙은 트리거나 코드가 맡는다. 이 열을 훑으면 DB가 보장하지 않는 자리가 어디인지 바로 보인다.

| 테이블 | 제약조건 | 적용 |
|:---|:---|:---|
| 모든 테이블 | 시각 컬럼은 `_at`으로 끝나고 `YYYY-MM-DDTHH:MM:SSZ` 형식 | `CHECK` |
| 모든 테이블 | enum, 금액, 불리언은 허용 범위 안의 값만 | `CHECK` |
| `contact` | 이름 필수 | `NOT NULL` |
| `contact` | 이메일·전화번호 중 하나 이상 | `CHECK` |
| `contact` | 관계처별 보관되지 않은 주 담당자는 최대 한 명 | 부분 유니크 인덱스 |
| `opportunity` | `(pipeline, stage)`는 `pipeline_stage`의 같은 쌍을 참조 | FK |
| `pipeline` | 기본키 `pipeline` | `PK` |
| `pipeline` | 참조된 행은 삭제 불가, 대신 `is_active`를 내림 | FK `RESTRICT` |
| `pipeline_stage` | `pipeline`은 `pipeline.pipeline`을 참조 | FK |
| `opportunity` | `lost_reason`은 `lost_reason.reason`을 참조 | FK |
| `opportunity` | 마감이 없으면 `due_at`, `due_time_zone`이 모두 `NULL`, 있으면 둘 다 필수 | `CHECK` |
| `opportunity` | `due_at`은 UTC RFC 3339 형식 | `CHECK` |
| `opportunity` | `due_time_zone`은 유효한 IANA 시간대 | 코드 |
| `opportunity` | 관계처가 있거나 `opportunity_contact`에 행이 하나 이상 | 트리거 |
| `opportunity` | 연락처를 붙일 때 그 연락처의 `account_id`가 진행 건과 일치 | 트리거 |
| `opportunity` | `lost_reason`은 `outcome`이 `lost`인 단계에서만 `NULL`이 아님 | 트리거 |
| `opportunity` | `base_amount_minor`와 `base_currency_code`는 `outcome`이 `won`·`lost`인 단계에서만 채우고 이후 변경 없음 | 트리거 |
| `opportunity_contact` | 기본키 `(opportunity_id, contact_id)` | `PK` |
| `opportunity_contact` | 진행 건마다 `is_primary`는 최대 한 명 | 부분 유니크 인덱스 |
| `pipeline_stage` | 기본키 `(pipeline, stage)` | `PK` |
| `pipeline_stage` | 파이프라인마다 `position` 중복 금지 | `UNIQUE` |
| `pipeline_stage` | 파이프라인마다 `outcome`이 `open`·`won`·`lost`인 단계가 각각 하나 이상 | 코드 |
| `pipeline_stage` | 그 단계를 쓰는 진행 건이 있으면 `outcome`을 바꿀 수 없음 | 트리거 |
| `lost_reason` | 기본키 `reason` | `PK` |
| `lost_reason` | 참조된 행은 삭제 불가, 대신 `is_active`를 내림 | FK `RESTRICT` |
| `activity` | 관계처·연락처·진행 건 중 하나 이상 | `CHECK` |
| `activity` | 진행 건에 연결하면 `account_id`·`business`가 일치하고 `contact_id`는 그 건의 연락처 중 하나 | 트리거 |
| `resource_link` | 같은 CRM 레코드와 외부 자원의 활성 중복 연결 금지 | 부분 유니크 인덱스 |

트리거를 쓰기 전에 `PRAGMA recursive_triggers`를 정해야 한다. SQLite 기본값은 꺼짐이고, 그 상태에서는 트리거 안의 `UPDATE`가 같은 트리거를 다시 발동시키지 않는다. 한 트리거가 고친 값을 다른 트리거가 이어받아 검사하기를 기대하고 짜면 조용히 건너뛴다. 켜면 무한 재귀를 직접 막아야 한다. 이 문서는 꺼짐을 전제로 하고, 연쇄가 필요한 규칙은 한 트리거 안에서 끝낸다.

`pipeline_stage`의 `outcome`을 바꾸지 못하게 막는 것도 같은 이유다. 진행 건이 그 단계에 있는 채로 결말이 바뀌면 `lost_reason`과 `base_amount_minor` 규칙을 어긴 행이 생기는데, `opportunity`를 아무도 건드리지 않았으므로 그쪽 트리거는 뜨지 않는다. 설정을 데이터로 둔 대가이고, 파이프라인 편집은 드문 일이라 막는 편이 정리하는 것보다 단순하다.

`due_time_zone`은 `CHECK`로 검증할 수 없다. SQLite에 시간대 데이터베이스가 없어서 `Asia/Seoul`이 실재하는 시간대인지 판별할 방법이 없다. 형식만 보는 `GLOB`은 오타를 못 잡으므로 저장 전에 코드가 확인한다.

부분 유니크 인덱스는 조건을 만족하는 행에만 유일성을 요구한다. 주 담당자와 주 연락처처럼 "하나만 허용"이 특정 값에만 걸릴 때 쓴다.

```sql
CREATE UNIQUE INDEX opportunity_primary_contact
  ON opportunity_contact (opportunity_id) WHERE is_primary = 1;
```

## v1에서 제외한 것과 도입 시점

| 제외한 것 | 도입 시점 |
|:---|:---|
| 관계처 유형·태그 조인 테이블 | 태그별 집계 질의가 JSON 검색으로 느려질 때 |
| 다중 소속 조인 테이블 | 한 사람이 동시에 두 곳에 속할 때. 순차적인 이직은 소속을 덮어써도 과거 활동과 진행 건이 각자 `account_id`를 들고 있어 이력이 남는다 |
| 단계 변경 이력 테이블 | 활동 기록만으로 파이프라인 분석이 부족할 때 |
| 구독 테이블 | 상품 서비스 조회로 감당이 안 될 때 |
| 행사 참여 테이블 | 참여 상태별 집계가 실제 요구로 올라올 때 |
| 연동 outbox 큐 | 외부 호출 실패율이 사용자에게 문제가 될 때 |
| 가져오기·내보내기 작업 테이블 | CSV 규모가 커져 동기 처리로 감당하기 어려울 때 |
| 감사 로그 테이블 | 필드 단위 이력 요구가 문서로 확정될 때 |
| 중복 병합 | 메일·캘린더 자동 수집이 붙어 중복이 사람 손보다 빠르게 쌓일 때. 패자를 지우지 않고 승자 ID를 남겨 예전 참조가 도착하게 한다 |
| 인덱스 계획 | 목록 화면의 정렬·필터 조건이 확정될 때. 지금 문서에 있는 것은 연락처 중복 검색용 표현식 인덱스와 `(pipeline, stage, stage_position)`뿐이고, 나머지는 실제 질의를 보고 붙인다 |
| 파이프라인 편집 화면 | 사용자가 직접 파이프라인이나 단계를 늘리고 줄이려 할 때. `pipeline`과 `pipeline_stage` 행을 고치면 되므로 화면만 붙이면 된다 |

보관된 레코드는 v1에서 자동 삭제하지 않는다. 주기적 정리 작업도 두지 않는다.
