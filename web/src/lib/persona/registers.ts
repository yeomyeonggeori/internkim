import registersDocument from '../../../../internal/admind/persona-schema/registers.json';
import type { Locale } from '$lib/i18n/locale';

export type SpeechRegister = (typeof registersDocument.registers)[number];

export const speechRegisters: SpeechRegister[] = registersDocument.registers;

export const defaultSpeechRegister = 'formal';

export function speechRegisterOf(value: string): SpeechRegister | undefined {
	return speechRegisters.find((register) => register.value === value);
}

export function speechRegisterName(value: string, locale: Locale): string {
	const register = speechRegisterOf(value);
	if (!register) return value;
	return locale === 'en' ? register.english.name : register.korean.name;
}
