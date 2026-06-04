type FlowWeek = {
	code: string;
	startISO: string;
	endISO: string;
	previous: string;
	next: string;
	isCurrent: boolean;
};

type FlowMember = {
	id: string;
	name: string;
	email: string;
	hireDate?: string;
	role: string;
	mattermostStatus: string;
	score: number;
	activeTaskCount: number;
	completeTaskCount: number;
};

type FlowTask = {
	id: string;
	ownerID: string;
	ownerName: string;
	participantIDs: string[];
	participantNames: string[];
	business: string;
	type: string;
	content: string;
	goal: string;
	size: string;
	status: string;
	startDate?: string;
	endDate?: string;
	weekCode: string;
	flag: number;
	requestReason?: string;
	decisionReason?: string;
};

type FlowMetrics = {
	totalTasks: number;
	completedTasks: number;
	requestedTasks: number;
	pausedTasks: number;
	stoppedTasks: number;
	totalScore: number;
	statusCounts: Record<string, number>;
	businessCounts: Record<string, number>;
	typeCounts: Record<string, number>;
	memberScores: Record<string, number>;
};

type FlowSizeDefinition = {
	name: string;
	distanceKm: number;
	maxHours: number;
	developmentExample: string;
	otherExample: string;
	note: string;
	score: number;
	label: string;
};

type FlowDefinitions = {
	categories: string[];
	types: string[];
	sizes: FlowSizeDefinition[];
};

export type DevFlowSummary = {
	week: FlowWeek;
	members: FlowMember[];
	tasks: FlowTask[];
	metrics: FlowMetrics;
	definitions: FlowDefinitions;
	statusOptions: string[];
	currentUserEmail: string;
	currentUserName: string;
	isAdmin: boolean;
	source: string;
};

const flowTypes = ['기능', '개선', '변경', '수정', '기획', '디자인', '마케팅', '운영', '회의', '미팅', '문서', '기타'];
const flowStatuses = ['요청', '예정', '진행', '완료', '일시정지', '기각', '중단'];

const flowSizes: FlowSizeDefinition[] = [
	sizeDefinition('XS', 1, 1, '아주 사소한 변경', '전화 / 10분 회의 / 전달 / 정리 / 일정 조율', '잠깐이면 끝낼 것'),
	sizeDefinition('S', 2, 2, '난이도 낮고 영향 범위 좁은 변경', '30분 내외 회의 / 간단 문서 초안 / 고객 및 파트너 대응 / 브리핑', '하루 여러 번도 처리 가능한 것'),
	sizeDefinition('M', 3, 8, '소형 기능 추가 / 영향 있는 변경', '보고서 작성 / 2시간 이내 회의 / 외부 미팅 / 팀 간 조율 / 문서 작성', '하루 날 잡고 해야 할 것'),
	sizeDefinition('L', 5, 16, '중형 기능 추가 / 다수 영향 있는 변경', '중요 외부 미팅 / 중형 리서치 / 기획서 초안 / 정책 변경', '이틀은 걸릴 것'),
	sizeDefinition('XL', 8, 32, '대형 기능 추가 / 복잡한 변경 / 외부 연동', '재정비 / 협상 / 장시간 미팅 / 워크샵', '일주일은 걸릴 것'),
	sizeDefinition('XXL', 13, 128, '마일스톤', '파트너십 설계 / 계약 구조 설계 / 서비스 기획 / 정책 개편', '반드시 하위 항목으로 쪼갤 것')
];

const fixtureMembers: FlowMember[] = [
	member('kim-intern', '김철수', 'admin@example.com', 'admin', '2026-03-02'),
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

export function createDevFlowSummary(weekCode: string | null | undefined, currentUserEmail = 'admin@example.com'): DevFlowSummary {
	const week = buildFlowWeek(weekCode);
	const tasks = createFixtureTasks(week);
	const definitions = { categories: ['개발', '운영', '기획', '디자인'], types: flowTypes, sizes: flowSizes };
	const members = scoreMembers(fixtureMembers, tasks, definitions);

	return {
		week,
		members,
		tasks,
		metrics: buildMetrics(tasks, definitions),
		definitions,
		statusOptions: flowStatuses,
		currentUserEmail,
		currentUserName: memberNameForEmail(members, currentUserEmail),
		isAdmin: true,
		source: 'dev-mock'
	};
}

function createFixtureTasks(week: FlowWeek): FlowTask[] {
	return [
		task('flow-dashboard', week, 'kim-intern', ['kim-intern', 'engineer'], '개발', '기능', 'Flow 주간 리포트 카드 정리', '업무 진행도 화면에서 상태와 거리 흐름을 빠르게 본다.', 'L', '진행', 0, 0),
		task('calendar-sync', week, 'engineer', ['engineer'], '개발', '개선', '캘린더 원격 동기화 재시도 점검', '반복 동기화 실패를 줄인다.', 'M', '완료', 1, 2),
		task('attendance-policy', week, 'operator', ['operator', 'kim-intern'], '운영', '기획', '근태 위치 정책 초안 작성', '오피스 출퇴근 판정 기준을 명확히 한다.', 'M', '요청', 2, 0, '운영팀 검토 후 진행'),
		task('mail-triage', week, 'kim-intern', ['kim-intern'], '운영', '문서', '메일 분류 규칙 정리', '반복 문의를 빠르게 triage한다.', 'S', '예정', 3, 0),
		task('design-pass', week, 'designer', ['designer'], '디자인', '디자인', 'Flow 모바일 간격 점검', '모바일에서 카드와 표가 안정적으로 보이게 한다.', 'S', '완료', 4, 4),
		task('mattermost-smoke', week, 'operator', ['operator', 'engineer'], '운영', '검증', 'Mattermost smoke 시나리오 재정리', '실서버 배포 전 수동 확인 범위를 줄인다.', 'M', '일시정지', 2, 0, '실서버 계정 준비 대기'),
		task('memory-graph', week, 'engineer', ['engineer', 'kim-intern'], '개발', '기능', 'Memory 그래프 빈 상태 처리', 'Blueclaw 연결 전에도 화면이 깨지지 않게 한다.', 'L', '중단', 5, 0, 'Blueclaw API 계약 변경 대기'),
		task('launch-brief', week, 'designer', ['designer', 'operator'], '기획', '문서', '내부 데모 브리프 작성', '팀이 데모 흐름을 같은 순서로 볼 수 있게 한다.', 'XS', '완료', 1, 3),
		task('roadmap-review', week, 'planner', ['planner'], '기획', '기획', '다음 스프린트 로드맵 정리', '우선순위와 진행 순서를 정리한다.', 'M', '진행', 1, 0),
		task('campaign-copy', week, 'marketer', ['marketer'], '기획', '마케팅', '온보딩 캠페인 문구 작성', '신규 사용자가 첫 업무를 빠르게 등록하게 한다.', 'S', '완료', 2, 4),
		task('regression-check', week, 'qa', ['qa'], '개발', '검증', 'Flow 회귀 테스트 체크리스트 실행', '보고 탭 주요 흐름을 배포 전에 확인한다.', 'M', '완료', 3, 5),
		task('customer-reply', week, 'support', ['support'], '운영', '운영', '고객 문의 답변 정리', '반복 문의 답변을 표준화한다.', 'S', '진행', 2, 0),
		task('release-note', week, 'writer', ['writer'], '운영', '문서', '릴리즈 노트 초안 작성', '팀 변경 사항을 읽기 쉽게 정리한다.', 'S', '완료', 4, 5),
		task('market-scan', week, 'researcher', ['researcher'], '기획', '기타', '경쟁 서비스 화면 조사', '업무 대시보드 비교 기준을 수집한다.', 'M', '진행', 0, 0)
	];
}

function task(
	id: string,
	week: FlowWeek,
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
): FlowTask {
	const owner = memberByID(ownerID);
	const participants = participantIDs.map(memberByID);

	return {
		id,
		ownerID,
		ownerName: owner.name,
		participantIDs,
		participantNames: participants.map((participant) => participant.name),
		business,
		type,
		content,
		goal,
		size,
		status,
		startDate: addDays(week.startISO, startOffset),
		endDate: endOffset > 0 ? addDays(week.startISO, endOffset) : '',
		weekCode: week.code,
		flag: status === '중단' || status === '일시정지' ? 1 : 0,
		requestReason,
		decisionReason: ''
	};
}

function buildMetrics(tasks: FlowTask[], definitions: FlowDefinitions): FlowMetrics {
	const metrics: FlowMetrics = {
		totalTasks: 0,
		completedTasks: 0,
		requestedTasks: 0,
		pausedTasks: 0,
		stoppedTasks: 0,
		totalScore: 0,
		statusCounts: {},
		businessCounts: {},
		typeCounts: {},
		memberScores: {}
	};

	for (const task of tasks) {
		metrics.totalTasks += 1;
		increment(metrics.statusCounts, task.status, 1);
		increment(metrics.businessCounts, task.business, 1);
		increment(metrics.typeCounts, task.type, 1);
		if (task.status === '완료') metrics.completedTasks += 1;
		if (task.status === '요청') metrics.requestedTasks += 1;
		if (task.status === '일시정지') metrics.pausedTasks += 1;
		if (task.status === '중단') metrics.stoppedTasks += 1;

		const score = scoreForTask(task, definitions);
		metrics.totalScore += score;
		for (const participantName of task.participantNames) {
			increment(metrics.memberScores, participantName, score);
		}
	}

	return metrics;
}

function scoreMembers(members: FlowMember[], tasks: FlowTask[], definitions: FlowDefinitions): FlowMember[] {
	return members.map((member) => {
		const memberTasks = tasks.filter((task) => task.participantIDs.includes(member.id));
		return {
			...member,
			score: memberTasks.reduce((total, task) => total + scoreForTask(task, definitions), 0),
			activeTaskCount: memberTasks.filter((task) => task.status !== '완료' && task.status !== '기각' && task.status !== '중단').length,
			completeTaskCount: memberTasks.filter((task) => task.status === '완료').length
		};
	});
}

function scoreForTask(task: FlowTask, definitions: FlowDefinitions): number {
	const distance = definitions.sizes.find((size) => size.name === task.size)?.distanceKm ?? 0;
	if (task.status === '완료') return distance;
	if (task.status === '진행') return Math.floor(distance / 2);
	return 0;
}

function buildFlowWeek(inputWeekCode: string | null | undefined): FlowWeek {
	const fallbackStart = mondayForWeekCode('26W23');
	const start = inputWeekCode ? mondayForWeekCode(inputWeekCode) : fallbackStart;
	const code = weekCodeForMonday(start);

	return {
		code,
		startISO: dateISO(start),
		endISO: dateISO(addDate(start, 6)),
		previous: weekCodeForMonday(addDate(start, -7)),
		next: weekCodeForMonday(addDate(start, 7)),
		isCurrent: code === weekCodeForMonday(fallbackStart)
	};
}

function mondayForWeekCode(weekCode: string): Date {
	const matched = /^(\d{2})W(\d{1,2})$/.exec(weekCode.trim());
	if (!matched) return mondayForWeekCode('26W23');

	const year = 2000 + Number(matched[1]);
	const week = Number(matched[2]);
	const januaryFourth = new Date(Date.UTC(year, 0, 4));
	const isoWeekOneMonday = addDate(januaryFourth, -((januaryFourth.getUTCDay() + 6) % 7));
	return addDate(isoWeekOneMonday, (week - 1) * 7);
}

function weekCodeForMonday(monday: Date): string {
	const thursday = addDate(monday, 3);
	const year = thursday.getUTCFullYear();
	const januaryFourth = new Date(Date.UTC(year, 0, 4));
	const isoWeekOneMonday = addDate(januaryFourth, -((januaryFourth.getUTCDay() + 6) % 7));
	const week = Math.floor((monday.getTime() - isoWeekOneMonday.getTime()) / (7 * 24 * 60 * 60 * 1000)) + 1;
	return `${String(year).slice(2)}W${String(week).padStart(2, '0')}`;
}

function member(id: string, name: string, email: string, role: string, hireDate: string): FlowMember {
	return { id, name, email, role, hireDate, mattermostStatus: 'active', score: 0, activeTaskCount: 0, completeTaskCount: 0 };
}

function memberByID(id: string): FlowMember {
	const found = fixtureMembers.find((member) => member.id === id);
	if (!found) throw new Error(`unknown dev flow member: ${id}`);
	return found;
}

function memberNameForEmail(members: FlowMember[], email: string): string {
	return members.find((member) => member.email === email)?.name ?? email.split('@')[0] ?? '';
}

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

function increment(record: Record<string, number>, key: string, amount: number): void {
	record[key] = (record[key] ?? 0) + amount;
}

function addDays(isoDate: string, days: number): string {
	return dateISO(addDate(new Date(`${isoDate}T00:00:00.000Z`), days));
}

function addDate(date: Date, days: number): Date {
	const nextDate = new Date(date);
	nextDate.setUTCDate(nextDate.getUTCDate() + days);
	return nextDate;
}

function dateISO(date: Date): string {
	return date.toISOString().slice(0, 10);
}
