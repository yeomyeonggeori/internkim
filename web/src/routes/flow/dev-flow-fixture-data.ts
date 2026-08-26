import type { FlowMember, FlowSizeDefinition } from './flow-types';

export type DevFlowTaskSpec = {
	id: string;
	parentTaskID?: string;
	requesterID?: string;
	ownerID: string;
	participantIDs: string[];
	business: string;
	type: string;
	content: string;
	size: string;
	status: string;
	startOffset: number;
	endOffset: number;
};

export const devFlowTypes = ['운동', '마케팅', '데이터 분석', '운영', '기획', '미팅', '회의', '문서', '보안', '연동', '변경', '설계', 'UI', '기능 추가', '리팩터링', '버그 수정', '기능', '개선', '검증', '디자인', '기타'];
export const devFlowStatuses = ['요청', '예정', '진행', '완료', '일시정지', '기각', '중단'];
export const devFlowBaselineWeekStartISO = '2026-06-01';

export const devFlowMembers: FlowMember[] = [
	member('kim-intern', '김철수', 'kim@example.com', 'admin', '2026-03-02'),
	member('designer', '이영희', 'designer@example.com', 'member', '2026-03-09'),
	member('engineer', '박예시', 'engineer@example.com', 'member', '2026-03-16'),
	member('operator', '최서연', 'operator@example.com', 'member', '2026-03-23'),
	member('planner', '정하준', 'planner@example.com', 'member', '2026-03-30'),
	member('marketer', '한지민', 'marketer@example.com', 'member', '2026-04-06'),
	member('qa', '윤도현', 'qa@example.com', 'member', '2026-04-13'),
	member('support', '강서준', 'support@example.com', 'member', '2026-04-20'),
	member('writer', '오하린', 'writer@example.com', 'member', '2026-04-27'),
	member('researcher', '임수아', 'researcher@example.com', 'member', '2026-05-04')
];

export const devFlowTaskSpecs: DevFlowTaskSpec[] = [
	taskSpec('flow-dashboard', 'kim-intern', ['kim-intern', 'engineer'], '여명거리', '기능', 'Flow 주간 리포트 카드 정리', 'L', '진행', 0, 0, { requesterID: 'kim-intern' }),
	taskSpec('calendar-sync', 'engineer', ['engineer'], '여명거리', '개선', '캘린더 원격 동기화 재시도 점검', 'M', '완료', 1, 2, { parentTaskID: 'flow-dashboard' }),
	taskSpec('attendance-policy', 'operator', ['operator', 'kim-intern'], '김인턴', '기획', '근태 위치 정책 초안 작성', 'M', '요청', 2, 0, { parentTaskID: 'flow-dashboard' }),
	taskSpec('mail-triage', 'kim-intern', ['kim-intern'], '김인턴', '문서', '메일 분류 규칙 정리', 'S', '예정', 3, 0, { parentTaskID: 'flow-dashboard' }),
	taskSpec('design-pass', 'designer', ['designer'], '여명거리', '디자인', 'Flow 모바일 간격 점검', 'S', '완료', 4, 4, { parentTaskID: 'flow-dashboard' }),
	taskSpec('mattermost-smoke', 'operator', ['operator', 'engineer'], '김인턴', '검증', 'Mattermost smoke 시나리오 재정리', 'M', '일시정지', 2, 0, { parentTaskID: 'flow-dashboard' }),
	taskSpec('memory-graph', 'engineer', ['engineer', 'kim-intern'], '여명거리', '기능', 'Memory 그래프 빈 상태 처리', 'L', '중단', 5, 0, { parentTaskID: 'flow-dashboard' }),
	taskSpec('launch-brief', 'designer', ['designer', 'operator'], '김인턴', '문서', '내부 데모 브리프 작성', 'XS', '완료', 1, 3),
	taskSpec('roadmap-review', 'planner', ['planner'], '여명거리', '기획', '다음 스프린트 로드맵 정리', 'M', '진행', 1, 0),
	taskSpec('campaign-copy', 'marketer', ['marketer'], '김인턴', '마케팅', '온보딩 캠페인 문구 작성', 'S', '완료', 2, 4),
	taskSpec('regression-check', 'qa', ['qa'], '여명거리', '검증', 'Flow 회귀 테스트 체크리스트 실행', 'M', '완료', 3, 5),
	taskSpec('customer-reply', 'support', ['support'], '김인턴', '운영', '고객 문의 답변 정리', 'S', '진행', 2, 0, { parentTaskID: 'flow-dashboard' }),
	taskSpec('release-note', 'writer', ['writer', 'kim-intern'], '여명거리', '문서', '릴리즈 노트 초안 작성', 'S', '완료', 4, 5),
	taskSpec('market-scan', 'researcher', ['researcher'], '김인턴', '기타', '경쟁 서비스 화면 조사', 'M', '기각', 0, 0, { parentTaskID: 'flow-dashboard' })
];

export const devFlowFutureTaskSpecs: DevFlowTaskSpec[] = [
	taskSpec('partner-demo', 'planner', ['planner', 'marketer'], '여명거리', '미팅', '파트너 데모 리허설', 'S', '완료', 0, 1),
	taskSpec('qa-followup', 'qa', ['qa', 'engineer'], '김인턴', '검증', 'Flow 보고 그래프 재확인', 'M', '진행', 2, 0),
	taskSpec('release-review', 'writer', ['writer'], '여명거리', '문서', '릴리즈 문구 검수', 'M', '완료', 4, 4),
	taskSpec('support-cleanup', 'support', ['support', 'operator'], '김인턴', '운영', '운영 문의 정리', 'S', '완료', 5, 5)
];

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
	size: string,
	status: string,
	startOffset: number,
	endOffset: number,
	relationships: Pick<DevFlowTaskSpec, 'parentTaskID' | 'requesterID'> = {}
): DevFlowTaskSpec {
	return { id, ownerID, participantIDs, business, type, content, size, status, startOffset, endOffset, ...relationships };
}
