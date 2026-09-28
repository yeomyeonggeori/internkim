export function relativeTime(isoTimestamp: string): string {
	const elapsedSeconds = Math.max(0, Math.round((Date.now() - new Date(isoTimestamp).getTime()) / 1000));
	if (elapsedSeconds < 60) return '방금';
	const elapsedMinutes = Math.round(elapsedSeconds / 60);
	if (elapsedMinutes < 60) return `${elapsedMinutes}분 전`;
	const elapsedHours = Math.round(elapsedMinutes / 60);
	if (elapsedHours < 24) return `${elapsedHours}시간 전`;
	return `${Math.round(elapsedHours / 24)}일 전`;
}

export function clockTime(isoTimestamp: string): string {
	return new Date(isoTimestamp).toLocaleTimeString('ko-KR', {
		hour: 'numeric',
		minute: '2-digit',
		hour12: true
	});
}

export function dateKeyOf(isoTimestamp: string): string {
	return new Date(isoTimestamp).toLocaleDateString('en-CA');
}

export function dateLabel(isoTimestamp: string): string {
	return new Date(isoTimestamp).toLocaleDateString('ko-KR', {
		year: 'numeric',
		month: 'long',
		day: 'numeric',
		weekday: 'short'
	});
}
