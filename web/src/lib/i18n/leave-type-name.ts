import type { Locale } from './locale.svelte';

const defaultLeaveTypeNames = {
	annual: { ko: '연차', en: 'Annual leave' },
	sick: { ko: '병가', en: 'Sick leave' },
	bereavement: { ko: '경조휴가', en: 'Bereavement leave' },
	public: { ko: '공가', en: 'Official leave' },
	maternity: { ko: '출산휴가', en: 'Maternity leave' },
	'spouse-maternity': { ko: '배우자 출산휴가', en: 'Spouse maternity leave' },
	'miscarriage-stillbirth': { ko: '유산·사산휴가', en: 'Miscarriage and stillbirth leave' },
	'fertility-treatment': { ko: '난임치료휴가', en: 'Fertility treatment leave' },
	'family-care': { ko: '가족돌봄휴가', en: 'Family care leave' },
	reward: { ko: '포상휴가', en: 'Reward leave' },
	compensatory: { ko: '보상휴가', en: 'Compensatory leave' },
	'long-service': { ko: '장기근속휴가', en: 'Long-service leave' },
	refresh: { ko: '리프레시휴가', en: 'Refresh leave' },
	'parental-leave': { ko: '육아휴직', en: 'Parental leave' },
	unpaid: { ko: '무급휴가', en: 'Unpaid leave' },
	other: { ko: '기타 휴가', en: 'Other leave' }
} as const;

export function localizedLeaveTypeName(id: string, name: string, locale: Locale): string {
	const names = defaultLeaveTypeNames[id as keyof typeof defaultLeaveTypeNames];
	if (!names) return name;
	const normalizedName = name.trim();
	if (normalizedName !== names.ko && normalizedName !== names.en) return name;
	return names[locale];
}
