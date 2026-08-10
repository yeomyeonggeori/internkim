export function windowOf(month: string): { from: string; to: string } {
	const [year, index] = month.split('-').map(Number);
	const next = index === 12 ? `${year + 1}-01` : `${year}-${String(index + 1).padStart(2, '0')}`;
	return { from: `${month}-01T00:00:00.000Z`, to: `${next}-01T00:00:00.000Z` };
}
