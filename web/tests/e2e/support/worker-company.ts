export const workerCompanyCount = 8;

const exampleCompanySlug = 'example-co';

export const workerCompanyNumber = Number(process.env.TEST_PARALLEL_INDEX ?? 0);

if (!Number.isInteger(workerCompanyNumber) || workerCompanyNumber >= workerCompanyCount) {
	throw new Error(`worker ${workerCompanyNumber} has no seeded company: supabase/seed.dev.sql seeds ${workerCompanyCount}`);
}

export function idInCompany(exampleID: string, companyNumber: number): string {
	return `${exampleID.slice(0, 19)}${String(companyNumber).padStart(4, '0')}${exampleID.slice(23)}`;
}

export function emailInCompany(exampleEmail: string, companyNumber: number): string {
	if (companyNumber === 0) return exampleEmail;
	return exampleEmail.replace('@', `+company${companyNumber}@`);
}

export function slugOfCompany(companyNumber: number): string {
	return companyNumber === 0 ? exampleCompanySlug : `${exampleCompanySlug}-${companyNumber}`;
}

export const inWorkerCompany = (exampleID: string): string => idInCompany(exampleID, workerCompanyNumber);
export const workerCompanyEmail = (exampleEmail: string): string => emailInCompany(exampleEmail, workerCompanyNumber);
export const workerCompanySlug = slugOfCompany(workerCompanyNumber);
