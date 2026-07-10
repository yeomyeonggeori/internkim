export type TimeTextSize = 'inherit' | 'extraSmall' | 'small' | 'medium';
export type TimeTextTone = 'inherit' | 'default' | 'muted' | 'info' | 'popover';

const sizeClasses: Record<TimeTextSize, string> = {
	inherit: '',
	extraSmall: 'text-[10px] leading-tight',
	small: 'text-xs',
	medium: 'text-sm'
};

const toneClasses: Record<TimeTextTone, string> = {
	inherit: '',
	default: 'text-foreground',
	muted: 'text-muted-foreground',
	info: 'text-info',
	popover: 'text-popover-foreground'
};

export function timeTextClasses(size: TimeTextSize, tone: TimeTextTone): string {
	return `${sizeClasses[size]} ${toneClasses[tone]}`.trim();
}
