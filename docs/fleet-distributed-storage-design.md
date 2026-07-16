# Fleet 통합 스토리지 설계

## 요약

김인턴에는 토렌트나 범용 분산 파일시스템보다 fleet-native
content-addressed storage가 적합하다.

- 기존 개인, circle, shared 논리 경로와 POSIX 권한은 유지한다.
- 파일 내용만 여러 기기의 여유 공간에 분산한다.
- 일반적인 10대 이하 fleet에서는 복제를 사용하고, erasure coding은 안정적인
  노드가 6대 이상인 대용량 cold 데이터에만 선택 적용한다.
- 원격 파일은 열거나 작업을 시작할 때 자동 hydrate해 기존 로컬 workspace처럼
  사용한다.
- 물리적인 50:50 파티션 대신 개인 최소 보장량과 shared 동적 quota를 사용한다.

## 핵심 구조

### Control plane

- 1대 fleet은 단독 metadata leader로 완전히 동작하게 한다.
- 2대 fleet은 두 번째 노드를 storage replica와 metadata mirror로 사용하되 자동
  quorum failover는 제공하지 않는다.
- 3대 이상부터 가장 안정적인 3대를 metadata voter로 선정한다. 규모가 커져도
  일반적으로 voter는 3대, 매우 큰 fleet만 5대로 제한한다.
- 사용자 기기의 storage 참여 여부와 metadata 투표권을 분리해, 기기가 꺼져도
  quorum이 불필요하게 흔들리지 않게 한다.
- 기존 rendezvous hashing, workspace bundle, quorum ledger 기반을 확장한다.

### Data plane

- 파일을 immutable chunk로 분할하고 SHA-256 기반 object ID로 저장한다.
- 작은 파일은 묶음 object로 저장해 filesystem 및 metadata 오버헤드를 줄인다.
- manifest가 논리 경로, 버전, ACL, 전체 크기, chunk 목록, 암호화 정보와
  placement 목표를 가진다.
- 쓰기는 local staging, chunk 배치, 복제 목표 확인, ledger의 path head 원자 교체
  순서로 publish한다.
- 읽기는 local chunk를 우선 사용하고, 없으면 provider index에서 가까운 노드를
  골라 병렬 hydrate한다.
- raw POSIX remote mount는 사용하지 않는다. `WorkspaceActor`가 파일 접근 및
  `terminal.run` 전에 필요한 working set을 hydrate한다.
- 모든 chunk 보유 노드를 전역 mesh로 연결하지 않고, anchor의 provider index와
  rendezvous placement로 탐색 비용을 제한한다.

### 노드 수별 정책

| Fleet 규모 | 기본 저장 정책 |
|---|---|
| 1대 | local 저장. 기기 장애 내구성은 제공하지 않음 |
| 2대 | durable 데이터 2복제. leader 장애 시 명시적 승격 |
| 3~5대 | 3 voter, durable 2복제, 중요 데이터 3복제 |
| 6~10대 | 동일한 복제 정책을 기본으로 하고 cold 대용량 데이터만 선택적 `4+2` erasure coding |
| 10대 초과 | voter 수는 제한하고 storage placement만 수평 확장 |

Erasure coding은 최소 `k+m`개의 독립 failure domain이 필요하고 작은 파일에는
증폭 비용이 있으므로 기본값으로 사용하지 않는다. `4+2`는 안정적인 storage
노드 6대 이상, 충분한 복구 여유 공간, 일정 크기 이상의 immutable cold object일
때만 허용한다.

### 데이터 등급과 quota

- `tmp`, socket, lock은 local ephemeral로 취급하고 복제하지 않는다.
- runtime 및 dependency cache는 다시 받을 수 있는 shared cache로 취급하고 저장
  공간이 부족할 때 우선 제거한다.
- 개인 artifact 및 circle/shared 파일은 기본 2복제로 저장한다.
- 중요 문서와 sealed snapshot은 3복제 또는 별도 중요도 정책을 적용한다.
- session log는 append-only segment로 동기화한다.
- DB는 실행 중인 파일을 shard하지 않고 sealed snapshot과 transaction log 단위로
  보호한다.
- 고정 파티션 대신 노드별 개인 최소 보장량, shared 최대치, 전체 high/low
  watermark를 둔다.
- shared cache는 자유롭게 제거할 수 있지만 durable replica는 다른 두 복제본이
  healthy인 경우에만 제거한다.
- 마지막 healthy replica가 있는 노드는 drain 완료 전 탈퇴하거나 초기화할 수
  없다.

## 인터페이스와 보안

- storage membership에 용량, 사용량, online 상태, 안정성 등급, failure domain,
  마지막 확인 시간을 추가한다.
- `ObjectManifest`, `ChunkReference`, `ReplicaPlacement`, `StorageNodeStatus`,
  `HydrationResult` 타입을 도입한다.
- 내부 작업은 `publish`, `resolve`, `hydrate`, `pin`, `evict`, `repair`, `drain`으로
  구분한다.
- peer 전송 API는 fleet 인증, chunk hash 검증, range read, 재시도와 속도 제한을
  지원한다.
- chunk는 fleet 내부에서도 암호화해 저장하고, object key는 ACL별 envelope로
  배포한다.
- object ID에는 fleet 비밀값으로 keyed hash를 적용해 다른 fleet과의 내용 동일성
  노출을 막는다.
- 논리 ACL은 기존 개인 및 circle POSIX 모델을 source of truth로 사용하며,
  물리적으로 다른 사용자 기기에 저장됐다는 이유로 접근 권한을 부여하지 않는다.
- IPFS와 토렌트는 content addressing과 multi-provider 전송 개념만 참고한다.
  provider가 모두 꺼지면 조회할 수 있으므로 pin과 복제 정책은 별도로 유지한다.

## 테스트 및 수용 기준

- 1, 2, 3, 5, 10, 100개 노드 topology에서 동일한 논리 경로가 안정적으로
  resolve된다.
- 같은 content는 fleet 내에서 한 번만 저장되고 ACL이 다른 참조도 올바르게
  분리된다.
- 파일 publish 도중 노드가 꺼지면 이전 path head가 유지된다.
- chunk 한 개가 손상되면 hash 검증으로 거부하고 다른 provider에서 복구한다.
- 두 storage 노드 동시 손실 후에도 durable 파일을 읽고 자동 repair할 수 있다.
- metadata quorum이 없는 partition은 새로운 publish를 확정하지 못한다.
- 원격 파일을 대상으로 `file.read`, artifact 작업, `terminal.run`을 호출하면 자동
  hydrate 후 정상 실행된다.
- 개인 권한이 없는 사용자는 local에 ciphertext chunk가 있어도 파일을 해독하거나
  조회하지 못한다.
- quota 압박 시 ephemeral 및 cache만 먼저 제거되고 durable replica 목표는
  유지된다.
- 노드 추가 및 drain 시 전체 데이터를 즉시 재배치하지 않고 필요한 범위만
  점진적으로 이동한다.

## 전제

- 일반적인 fleet은 10대 이하이지만 1대부터 제한 없이 수평 확장 가능해야 한다.
- 회사마다 최소 한 대의 상시 anchor가 있다.
- 개인 workspace의 암호화된 blob을 다른 회사 기기에 저장할 수 있다.
- durable 데이터는 두 대의 동시 storage 노드 손실을 견뎌야 한다.
- 회사 전체가 소실되는 사고를 위한 offsite backup은 이번 범위에서 제외한다.

## 참고 자료

- [Ceph erasure coding](https://docs.ceph.com/en/umbrella/rados/operations/erasure-code/)
- [IPFS 데이터 수명주기](https://docs.ipfs.tech/concepts/lifecycle/)
- [SeaweedFS](https://github.com/seaweedfs/seaweedfs)
