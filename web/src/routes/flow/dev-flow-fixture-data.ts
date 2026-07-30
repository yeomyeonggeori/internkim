import type { FlowMember, FlowSizeDefinition } from './flow-types';

export type DevFlowTaskSpec = {
	id: string;
	ownerID: string;
	participantIDs: string[];
	business: string;
	type: string;
	content: string;
	goal: string;
	size: string;
	status: string;
	startOffset: number;
	endOffset: number;
	requestReason?: string;
};

export const devFlowTypes = ['운동', '마케팅', '데이터 분석', '운영', '기획', '미팅', '회의', '문서', '보안', '연동', '변경', '설계', 'UI', '기능 추가', '리팩터링', '버그 수정', '기능', '개선', '검증', '디자인', '기타'];
export const devFlowStatuses = ['요청', '예정', '진행', '완료', '일시정지', '기각', '중단'];
export const devFlowBaselineWeekStartISO = '2026-06-01';

export const devFlowSizes: FlowSizeDefinition[] = [
	sizeDefinition('XS', 1, 1, '아주 사소한 변경', '전화 / 10분 회의 / 전달 / 정리 / 일정 조율', '잠깐이면 끝낼 것'),
	sizeDefinition('S', 2, 2, '난이도 낮고 영향 범위 좁은 변경', '30분 내외 회의 / 간단 문서 초안 / 고객 및 파트너 대응 / 브리핑', '하루 여러 번도 처리 가능한 것'),
	sizeDefinition('M', 3, 8, '소형 기능 추가 / 영향 있는 변경', '보고서 작성 / 2시간 이내 회의 / 외부 미팅 / 팀 간 조율 / 문서 작성', '하루 날 잡고 해야 할 것'),
	sizeDefinition('L', 5, 16, '중형 기능 추가 / 다수 영향 있는 변경', '중요 외부 미팅 / 중형 리서치 / 기획서 초안 / 정책 변경', '이틀은 걸릴 것'),
	sizeDefinition('XL', 8, 32, '대형 기능 추가 / 복잡한 변경 / 외부 연동', '재정비 / 협상 / 장시간 미팅 / 워크샵', '일주일은 걸릴 것'),
	sizeDefinition('XXL', 13, 128, '마일스톤', '파트너십 설계 / 계약 구조 설계 / 서비스 기획 / 정책 개편', '반드시 하위 항목으로 쪼갤 것')
];

export const devFlowMembers: FlowMember[] = [
	member('kim-intern', '김철수', 'kim@example.com', 'admin', '2026-03-02'),
	member('designer', '이영희', 'designer@example.com', 'member', '2026-03-09'),
	member('engineer', '박민준', 'engineer@example.com', 'member', '2026-03-16'),
	member('operator', '최서연', 'operator@example.com', 'member', '2026-03-23'),
	member('planner', '정하준', 'planner@example.com', 'member', '2026-03-30'),
	member('marketer', '한지민', 'marketer@example.com', 'member', '2026-04-06'),
	member('qa', '윤도현', 'qa@example.com', 'member', '2026-04-13'),
	member('support', '강서준', 'support@example.com', 'member', '2026-04-20'),
	member('writer', '오하린', 'writer@example.com', 'member', '2026-04-27'),
	member('researcher', '임수아', 'researcher@example.com', 'member', '2026-05-04')
];

export const devFlowTaskSpecs: DevFlowTaskSpec[] = [
	taskSpec('flow-dashboard', 'kim-intern', ['kim-intern', 'engineer'], '여명거리', '기능', 'Flow 주간 리포트 카드 정리', '업무 진행도 화면에서 상태와 거리 흐름을 빠르게 본다.', 'L', '진행', 0, 0),
	taskSpec('calendar-sync', 'engineer', ['engineer'], '여명거리', '개선', '캘린더 원격 동기화 재시도 점검', '반복 동기화 실패를 줄인다.', 'M', '완료', 1, 2),
	taskSpec('attendance-policy', 'operator', ['operator', 'kim-intern'], '김인턴', '기획', '근태 위치 정책 초안 작성', '오피스 출퇴근 판정 기준을 명확히 한다.', 'M', '요청', 2, 0, '운영팀 검토 후 진행'),
	taskSpec('mail-triage', 'kim-intern', ['kim-intern'], '김인턴', '문서', '메일 분류 규칙 정리', '반복 문의를 빠르게 triage한다.', 'S', '예정', 3, 0),
	taskSpec('design-pass', 'designer', ['designer'], '여명거리', '디자인', 'Flow 모바일 간격 점검', '모바일에서 카드와 표가 안정적으로 보이게 한다.', 'S', '완료', 4, 4),
	taskSpec('mattermost-smoke', 'operator', ['operator', 'engineer'], '김인턴', '검증', 'Mattermost smoke 시나리오 재정리', '실서버 배포 전 수동 확인 범위를 줄인다.', 'M', '일시정지', 2, 0, '실서버 계정 준비 대기'),
	taskSpec('memory-graph', 'engineer', ['engineer', 'kim-intern'], '여명거리', '기능', 'Memory 그래프 빈 상태 처리', 'Blueclaw 연결 전에도 화면이 깨지지 않게 한다.', 'L', '중단', 5, 0, 'Blueclaw API 계약 변경 대기'),
	taskSpec('launch-brief', 'designer', ['designer', 'operator'], '김인턴', '문서', '내부 데모 브리프 작성', '팀이 데모 흐름을 같은 순서로 볼 수 있게 한다.', 'XS', '완료', 1, 3),
	taskSpec('roadmap-review', 'planner', ['planner'], '여명거리', '기획', '다음 스프린트 로드맵 정리', '우선순위와 진행 순서를 정리한다.', 'M', '진행', 1, 0),
	taskSpec('campaign-copy', 'marketer', ['marketer'], '김인턴', '마케팅', '온보딩 캠페인 문구 작성', '신규 사용자가 첫 업무를 빠르게 등록하게 한다.', 'S', '완료', 2, 4),
	taskSpec('regression-check', 'qa', ['qa'], '여명거리', '검증', 'Flow 회귀 테스트 체크리스트 실행', '보고 탭 주요 흐름을 배포 전에 확인한다.', 'M', '완료', 3, 5),
	taskSpec('customer-reply', 'support', ['support'], '김인턴', '운영', '고객 문의 답변 정리', '반복 문의 답변을 표준화한다.', 'S', '진행', 2, 0),
	taskSpec('release-note', 'writer', ['writer'], '여명거리', '문서', '릴리즈 노트 초안 작성', '팀 변경 사항을 읽기 쉽게 정리한다.', 'S', '완료', 4, 5),
	taskSpec('market-scan', 'researcher', ['researcher'], '김인턴', '기타', '경쟁 서비스 화면 조사', '업무 대시보드 비교 기준을 수집한다.', 'M', '진행', 0, 0)
];

export const devFlowFutureTaskSpecs: DevFlowTaskSpec[] = [
	taskSpec('partner-demo', 'planner', ['planner', 'marketer'], '여명거리', '미팅', '파트너 데모 리허설', '외부 데모 전 흐름과 메시지를 맞춘다.', 'S', '완료', 0, 1),
	taskSpec('qa-followup', 'qa', ['qa', 'engineer'], '김인턴', '검증', 'Flow 보고 그래프 재확인', '주차 이동과 거리 그래프 반영을 확인한다.', 'M', '진행', 2, 0),
	taskSpec('release-review', 'writer', ['writer'], '여명거리', '문서', '릴리즈 문구 검수', '변경 내용을 팀이 같은 표현으로 안내한다.', 'M', '완료', 4, 4),
	taskSpec('support-cleanup', 'support', ['support', 'operator'], '김인턴', '운영', '운영 문의 정리', '반복 문의와 답변 기준을 정리한다.', 'S', '완료', 5, 5)
];

function sizeDefinition(name: string, distanceKm: number, maxHours: number, developmentExample: string, otherExample: string, note: string): FlowSizeDefinition {
	return {
		name,
		distanceKm,
		maxHours,
		developmentExample,
		otherExample,
		note,
		score: distanceKm,
		label: `${distanceKm}km · 최대 ${maxHours}h`
	};
}

function member(id: string, name: string, email: string, role: string, hireDate: string): FlowMember {
	return { id, name, email, role, hireDate, mattermostStatus: 'active', distance: 0, score: 0, activeTaskCount: 0, completeTaskCount: 0 };
}

function taskSpec(
	id: string,
	ownerID: string,
	participantIDs: string[],
	business: string,
	type: string,
	content: string,
	goal: string,
	size: string,
	status: string,
	startOffset: number,
	endOffset: number,
	requestReason = ''
): DevFlowTaskSpec {
	return { id, ownerID, participantIDs, business, type, content, goal, size, status, startOffset, endOffset, requestReason };
}
