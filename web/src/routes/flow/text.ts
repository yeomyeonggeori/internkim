import { enFlowText } from './text-en';
import { koFlowText } from './text-ko';

export const flowText = {
	ko: koFlowText,
	en: enFlowText
} as const;
