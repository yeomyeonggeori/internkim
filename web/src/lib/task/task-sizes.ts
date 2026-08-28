import type { TaskSizeDefinition } from '../../routes/task/task-types';

const sizes: { name: string; distanceKm: number; maxHours: number }[] = [
	{ name: 'XS', distanceKm: 1, maxHours: 1 },
	{ name: 'S', distanceKm: 2, maxHours: 2 },
	{ name: 'M', distanceKm: 3, maxHours: 8 },
	{ name: 'L', distanceKm: 5, maxHours: 16 },
	{ name: 'XL', distanceKm: 8, maxHours: 32 },
	{ name: 'XXL', distanceKm: 13, maxHours: 128 }
];

const korean: Record<string, { developmentExample: string; otherExample: string; note: string }> = {
	XS: { developmentExample: '아주 사소한 변경', otherExample: '전화 / 10분 회의 / 전달 / 정리 / 일정 조율', note: '잠깐이면 끝낼 것' },
	S: { developmentExample: '난이도 낮고 영향 범위 좁은 변경', otherExample: '30분 내외 회의 / 간단 문서 초안 / 고객 및 파트너 대응 / 브리핑', note: '하루 여러 번도 처리 가능한 것' },
	M: { developmentExample: '소형 기능 추가 / 영향 있는 변경', otherExample: '보고서 작성 / 2시간 이내 회의 / 외부 미팅 / 팀 간 조율 / 문서 작성', note: '하루 날 잡고 해야 할 것' },
	L: { developmentExample: '중형 기능 추가 / 다수 영향 있는 변경', otherExample: '중요 외부 미팅 / 중형 리서치 / 기획서 초안 / 정책 변경', note: '이틀은 걸릴 것' },
	XL: { developmentExample: '대형 기능 추가 / 복잡한 변경 / 외부 연동', otherExample: '재정비 / 협상 / 장시간 미팅 / 워크샵', note: '일주일은 걸릴 것' },
	XXL: { developmentExample: '마일스톤', otherExample: '파트너십 설계 / 계약 구조 설계 / 서비스 기획 / 정책 개편', note: '반드시 하위 항목으로 쪼갤 것' }
};

const english: Record<string, { developmentExample: string; otherExample: string; note: string }> = {
	XS: { developmentExample: 'Trivial change', otherExample: 'Call / 10 minute meeting / handoff / tidy-up / scheduling', note: 'Done in a moment' },
	S: { developmentExample: 'Low risk change with a narrow blast radius', otherExample: 'Half hour meeting / short draft / customer or partner reply / briefing', note: 'Several fit in a day' },
	M: { developmentExample: 'Small feature / change with real impact', otherExample: 'Report / meeting under two hours / external meeting / cross-team alignment / document', note: 'Takes a dedicated day' },
	L: { developmentExample: 'Mid-size feature / change touching many areas', otherExample: 'Important external meeting / mid-size research / proposal draft / policy change', note: 'Takes about two days' },
	XL: { developmentExample: 'Large feature / complex change / external integration', otherExample: 'Restructuring / negotiation / long meeting / workshop', note: 'Takes about a week' },
	XXL: { developmentExample: 'Milestone', otherExample: 'Partnership design / contract structure / service planning / policy overhaul', note: 'Must be split into smaller items' }
};

export function taskSizes(locale: 'ko' | 'en' = 'ko'): TaskSizeDefinition[] {
	const wording = locale === 'en' ? english : korean;
	return sizes.map((size) => ({
		name: size.name,
		distanceKm: size.distanceKm,
		maxHours: size.maxHours,
		score: size.distanceKm,
		label: locale === 'en' ? `${size.distanceKm}km · up to ${size.maxHours}h` : `${size.distanceKm}km · 최대 ${size.maxHours}h`,
		...wording[size.name]
	}));
}

export const taskSizeNames = sizes.map((size) => size.name);

export function sizeOfHours(hours: number): string {
	const fitting = sizes.find((size) => hours <= size.maxHours);
	return (fitting ?? sizes[sizes.length - 1]).name;
}

const sizeOfWholeDayCount = ['M', 'L', 'XL'];

export function sizeOfWholeDays(days: number): string {
	if (days < 1) return sizeOfWholeDayCount[0];
	return sizeOfWholeDayCount[days - 1] ?? 'XXL';
}
