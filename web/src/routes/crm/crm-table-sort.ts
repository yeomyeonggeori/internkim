export type CRMSortDirection = 'ascending' | 'descending';

export type CRMSortState = {
	key: string;
	direction: CRMSortDirection;
};

export type CRMSortValue = string | number | undefined;

export type CRMSortComparators<Row> = Record<string, (row: Row) => CRMSortValue>;

export type CRMSortTieBreaker<Row> = {
	read: (row: Row) => CRMSortValue;
	direction: CRMSortDirection;
};

export function nextSortState(current: CRMSortState | null, key: string): CRMSortState | null {
	if (current?.key !== key) return { key, direction: 'ascending' };
	if (current.direction === 'ascending') return { key, direction: 'descending' };
	return null;
}

function isEmpty(value: CRMSortValue): boolean {
	return value === undefined || value === '' || (typeof value === 'number' && Number.isNaN(value));
}

function compare(left: CRMSortValue, right: CRMSortValue): number {
	if (typeof left === 'number' && typeof right === 'number') return left - right;
	return String(left).localeCompare(String(right), 'ko');
}

export function sortRows<Row>(
	rows: Row[],
	sort: CRMSortState | null,
	comparators: CRMSortComparators<Row>,
	tieBreaker?: CRMSortTieBreaker<Row>
): Row[] {
	const readValue = sort ? comparators[sort.key] : undefined;
	if (!sort || !readValue) return rows;
	const direction = sort.direction === 'ascending' ? 1 : -1;
	return [...rows].sort((leftRow, rightRow) => {
		const ordered = orderBy(readValue, leftRow, rightRow, direction);
		if (ordered !== 0 || !tieBreaker) return ordered;
		return orderBy(tieBreaker.read, leftRow, rightRow, tieBreaker.direction === 'ascending' ? 1 : -1);
	});
}

function orderBy<Row>(
	read: (row: Row) => CRMSortValue,
	leftRow: Row,
	rightRow: Row,
	direction: number
): number {
	const left = read(leftRow);
	const right = read(rightRow);
	if (isEmpty(left) && isEmpty(right)) return 0;
	if (isEmpty(left)) return 1;
	if (isEmpty(right)) return -1;
	return compare(left, right) * direction;
}
