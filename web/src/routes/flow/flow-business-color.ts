import { flowProjectColor } from './flow-report-colors';

const unknownBusinessColor = '#64748b';

export function flowBusinessColor(business: string, categories: string[]): string {
	const index = categories.indexOf(business.trim());
	if (index < 0) return unknownBusinessColor;
	return flowProjectColor(index);
}

export function flowBusinessBadgeStyle(color: string): string {
	return `background: ${color}1f; color: ${color}; border-color: ${color}33;`;
}
