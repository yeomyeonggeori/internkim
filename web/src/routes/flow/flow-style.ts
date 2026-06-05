export function statusBadgeClass(status: string): string {
	switch (status) {
		case '완료':
			return 'bg-[#d4edbc] text-[#1f3826] border-transparent';
		case '진행':
			return 'bg-[#bfe1f6] text-[#0b3d63] border-transparent';
		case '예정':
			return 'bg-[#ffe5a0] text-[#473821] border-transparent';
		case '요청':
			return 'bg-[#e6cff2] text-[#3d1c52] border-transparent';
		case '일시정지':
			return 'bg-[#ffcfc9] text-[#5b1c14] border-transparent';
		case '기각':
		case '중단':
			return 'bg-[#f6c1bd] text-[#5b1c14] border-transparent';
		default:
			return 'bg-muted text-muted-foreground border-transparent';
	}
}

export function sizeBadgeClass(size: string): string {
	switch (size) {
		case 'XS':
			return 'bg-[#f1f3f4] text-[#3c4043] border-transparent';
		case 'S':
			return 'bg-[#d4edbc] text-[#1f3826] border-transparent';
		case 'M':
			return 'bg-[#bfe1f6] text-[#0b3d63] border-transparent';
		case 'L':
			return 'bg-[#ffe5a0] text-[#473821] border-transparent';
		case 'XL':
			return 'bg-[#ffcfc9] text-[#5b1c14] border-transparent';
		case 'XXL':
			return 'bg-[#f6c1bd] text-[#5b1c14] border-transparent';
		default:
			return 'bg-muted text-muted-foreground border-transparent';
	}
}

export function compareOptionalDate(left: string | undefined, right: string | undefined): number {
	const leftValue = left ?? '';
	const rightValue = right ?? '';
	if (leftValue === rightValue) return 0;
	if (!leftValue) return 1;
	if (!rightValue) return -1;
	return leftValue < rightValue ? -1 : 1;
}
