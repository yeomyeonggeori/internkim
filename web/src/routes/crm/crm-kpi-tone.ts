import type { CRMKPISegment } from './crm-kpi';

const neutralClasses = ['bg-foreground', 'bg-foreground/55', 'bg-foreground/30'];
const attentionClass = 'bg-destructive';
const remainderClass = 'bg-muted-foreground';

export type CRMKPISegmentFill = {
	segment: CRMKPISegment;
	fillClass: string;
	fillStyle: string;
	percent: number;
};

function fillOf(segment: CRMKPISegment, ladderPosition: number): Pick<CRMKPISegmentFill, 'fillClass' | 'fillStyle'> {
	if (segment.color !== undefined) return { fillClass: '', fillStyle: `background: ${segment.color}` };
	if (segment.tone === 'attention') return { fillClass: attentionClass, fillStyle: '' };
	return { fillClass: neutralClasses[ladderPosition] ?? remainderClass, fillStyle: '' };
}

export function segmentFillsOf(segments: CRMKPISegment[]): CRMKPISegmentFill[] {
	const total = segments.reduce((sum, segment) => sum + segment.value, 0);
	let ladderPosition = 0;
	return segments.map((segment) => {
		const fill = fillOf(segment, ladderPosition);
		if (segment.tone === 'neutral') ladderPosition += 1;
		return { segment, ...fill, percent: total === 0 ? 0 : (segment.value / total) * 100 };
	});
}
