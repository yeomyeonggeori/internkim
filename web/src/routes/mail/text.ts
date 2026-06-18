import { mailTextEn } from './text-en';
import { mailTextKo } from './text-ko';

export const mailText = {
	ko: mailTextKo,
	en: mailTextEn
} as const;
