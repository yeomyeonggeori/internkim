export type OrganizationTenure = {
	years: number;
	months: number;
};

export function organizationTenure(hireDate: string | undefined, today: Date): OrganizationTenure | undefined {
	const hiredAt = parseHireDate(hireDate);
	if (!hiredAt) return undefined;
	const elapsedMonths = countElapsedMonths(hiredAt, today);
	if (elapsedMonths < 0) return undefined;
	return { years: Math.floor(elapsedMonths / 12), months: elapsedMonths % 12 };
}

function parseHireDate(hireDate: string | undefined): Date | undefined {
	const trimmedHireDate = (hireDate ?? '').trim();
	if (!/^\d{4}-\d{2}-\d{2}$/.test(trimmedHireDate)) return undefined;
	const hiredAt = new Date(`${trimmedHireDate}T00:00:00`);
	return Number.isNaN(hiredAt.getTime()) ? undefined : hiredAt;
}

function countElapsedMonths(hiredAt: Date, today: Date): number {
	const wholeMonths = (today.getFullYear() - hiredAt.getFullYear()) * 12 + (today.getMonth() - hiredAt.getMonth());
	return today.getDate() >= hiredAt.getDate() ? wholeMonths : wholeMonths - 1;
}
