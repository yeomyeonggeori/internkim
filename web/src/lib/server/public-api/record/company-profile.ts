import type { CompanyProfileLegalAttribute, CompanyProfileResult } from '../catalog/company';
import type { Money } from '$lib/money';
import type { RecordPerson } from './people';

export type LocalizedText = Record<string, string>;

export type CompanyProfileDocument = {
	name?: LocalizedText;
	brandName?: LocalizedText;
	slogan?: LocalizedText;
	description?: LocalizedText;
	address?: LocalizedText;
	officeAddress?: LocalizedText;
	jurisdiction?: LocalizedText;
	bankAccount?: LocalizedText;
	legalAttributes?: Record<string, LocalizedText>;
	foundedDate?: string;
	capital?: Money;
	fiscalYearEnd?: string;
	employeeCount?: number;
	phone?: string;
	fax?: string;
	email?: string;
	website?: string;
	updatedAt?: string;
};

export type CompanyProfileUpdate = {
	language?: string;
	name?: string;
	brandName?: string;
	slogan?: string;
	description?: string;
	address?: string;
	officeAddress?: string;
	jurisdiction?: string;
	bankAccount?: string;
	legalAttributes?: string;
	foundedDate?: string;
	capital?: Money;
	fiscalYearEnd?: string;
	employeeCount?: number;
	phone?: string;
	fax?: string;
	email?: string;
	website?: string;
};

const localizedFields = [
	'name',
	'brandName',
	'slogan',
	'description',
	'address',
	'officeAddress',
	'jurisdiction',
	'bankAccount'
] as const;

const plainFields = ['foundedDate', 'fiscalYearEnd', 'phone', 'fax', 'email', 'website'] as const;

const coreLocalizedFields = ['name', 'address', 'bankAccount'] as const;

const defaultLanguage = 'en';

export function languageAsked(offered: string | undefined): string {
	return offered?.trim().toLowerCase() || defaultLanguage;
}

export function representativeTitleOf(language: string): string {
	return language === 'ko' ? '대표이사' : 'CEO';
}

export function profileOf(stored: unknown): CompanyProfileDocument {
	if (typeof stored !== 'object' || stored === null || Array.isArray(stored)) return {};
	return stored as CompanyProfileDocument;
}

export function companyProfileView(
	profile: CompanyProfileDocument,
	language: string,
	representative: RecordPerson | undefined
): CompanyProfileResult {
	const missingFields: string[] = [];
	const resolvedCore = (field: (typeof coreLocalizedFields)[number]): string => {
		const written = profile[field]?.[language]?.trim() ?? '';
		if (written) return written;
		missingFields.push(field);
		return anyLocalized(profile[field], language);
	};

	const view: CompanyProfileResult = {
		language,
		name: resolvedCore('name'),
		brandName: anyLocalized(profile.brandName, language),
		slogan: anyLocalized(profile.slogan, language),
		description: anyLocalized(profile.description, language),
		representative: representativeNamed(representative, missingFields),
		representativeTitle: representative?.jobTitle.trim() || representativeTitleOf(language),
		representativePersonID: representative?.personID ?? null,
		address: resolvedCore('address'),
		officeAddress: anyLocalized(profile.officeAddress, language),
		jurisdiction: anyLocalized(profile.jurisdiction, language),
		bankAccount: resolvedCore('bankAccount'),
		legalAttributes: legalAttributesIn(profile.legalAttributes, language),
		foundedDate: profile.foundedDate ?? '',
		capital: profile.capital ?? null,
		fiscalYearEnd: profile.fiscalYearEnd ?? '',
		employeeCount: profile.employeeCount ?? 0,
		phone: profile.phone ?? '',
		fax: profile.fax ?? '',
		email: profile.email ?? '',
		website: profile.website ?? '',
		missingFields,
		updatedAt: profile.updatedAt ?? ''
	};

	if (!view.phone.trim()) missingFields.push('phone');
	if (!view.email.trim()) missingFields.push('email');
	return view;
}

function representativeNamed(representative: RecordPerson | undefined, missingFields: string[]): string {
	if (representative) return representative.name;
	missingFields.push('representative');
	return '';
}

export function profileWithUpdate(
	profile: CompanyProfileDocument,
	update: CompanyProfileUpdate,
	language: string,
	writtenAt: Date
): CompanyProfileDocument {
	const written: CompanyProfileDocument = { ...profile };

	for (const field of localizedFields) {
		const offered = update[field];
		if (offered === undefined) continue;
		written[field] = localizedWith(written[field], language, offered);
	}

	const labels = labelsOffered(update.legalAttributes);
	if (labels) {
		written.legalAttributes = {
			...(written.legalAttributes ?? {}),
			[language]: labelsWith(written.legalAttributes?.[language] ?? {}, labels)
		};
	}

	for (const field of plainFields) {
		const offered = update[field];
		if (offered === undefined) continue;
		if (offered.trim()) written[field] = offered.trim();
		else delete written[field];
	}
	if (update.capital) written.capital = update.capital;
	if (update.employeeCount !== undefined && update.employeeCount > 0) {
		written.employeeCount = update.employeeCount;
	}

	written.updatedAt = writtenAt.toISOString().replace(/\.\d{3}Z$/, 'Z');
	return written;
}

function localizedWith(held: LocalizedText | undefined, language: string, offered: string): LocalizedText {
	const written: LocalizedText = { ...(held ?? {}) };
	if (offered.trim()) written[language] = offered.trim();
	else delete written[language];
	return written;
}

function labelsOffered(written: string | undefined): LocalizedText | null {
	const offered = written?.trim();
	if (!offered) return null;
	const parsed: unknown = JSON.parse(offered);
	if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) {
		throw new Error('legalAttributes is a JSON object of label to value');
	}
	const labels: LocalizedText = {};
	for (const [label, value] of Object.entries(parsed)) {
		if (typeof value !== 'string') {
			throw new Error(`legalAttributes ${label} is not a string`);
		}
		labels[label] = value;
	}
	return labels;
}

function labelsWith(held: LocalizedText, offered: LocalizedText): LocalizedText {
	const written: LocalizedText = { ...held };
	for (const [offeredLabel, offeredValue] of Object.entries(offered)) {
		const label = offeredLabel.trim();
		if (!label) continue;
		const value = offeredValue.trim();
		if (value) written[label] = value;
		else delete written[label];
	}
	return written;
}

function anyLocalized(text: LocalizedText | undefined, language: string): string {
	if (!text) return '';
	const asked = text[language]?.trim();
	if (asked) return asked;
	const english = text.en?.trim();
	if (english) return english;
	for (const value of Object.values(text)) {
		const written = value.trim();
		if (written) return written;
	}
	return '';
}

function legalAttributesIn(
	attributes: Record<string, LocalizedText> | undefined,
	language: string
): CompanyProfileLegalAttribute[] {
	if (!attributes) return [];
	for (const labels of [attributes[language], attributes.en, ...Object.values(attributes)]) {
		const printed = Object.entries(labels ?? {});
		if (printed.length > 0) return printed.map(([label, value]) => ({ label, value }));
	}
	return [];
}
