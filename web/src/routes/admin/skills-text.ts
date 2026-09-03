import type { PageText } from '$lib/i18n/page-text.svelte';

export const skillsText = {
	ko: {
		title: '스킬',
		description: '에이전트가 실제로 읽어들인 스킬과, 이 컴퓨터가 만족시키지 못한 스킬입니다.',
		refresh: '새로고침',
		loadError: '스킬 목록을 불러오지 못했습니다.',
		empty: '에이전트가 읽어들인 스킬이 없습니다.',
		loadedCount: '읽어들임 {count}개',
		unavailableCount: '사용 불가 {count}개',
		unavailableTitle: '사용 불가',
		unavailableDescription: '이 컴퓨터가 갖추지 못한 것이 있어 에이전트가 쓰지 않는 스킬입니다.',
		missingEnvironment: '없는 환경 변수',
		missingTools: '없는 도구',
		toolReferences: '사용하는 도구',
		rootLabel: '읽어온 위치'
	},
	en: {
		title: 'Skills',
		description: 'Which skills the agent actually loaded, and which ones this computer cannot satisfy.',
		refresh: 'Refresh',
		loadError: 'Could not load the skill inventory.',
		empty: 'The agent loaded no skills.',
		loadedCount: '{count} loaded',
		unavailableCount: '{count} unavailable',
		unavailableTitle: 'Unavailable',
		unavailableDescription: 'Skills the agent leaves out because this computer lacks something they need.',
		missingEnvironment: 'Missing environment',
		missingTools: 'Missing tools',
		toolReferences: 'Tools used',
		rootLabel: 'Read from'
	}
};

export type SkillsText = PageText<typeof skillsText>;
