import type { SupabaseClient } from '@supabase/supabase-js';
import { localeOf, type Locale } from '$lib/i18n/locale';
import { labelsOfVocabulary, type CompanyLabels } from './labels';
import { leaveKindsOfPolicy, leaveYearStartOfPolicy, type LeaveYearStart, type LeaveKind } from './leave';
import { peopleOfCompany, type RecordPerson } from './people';
import type { TaskLabelDecider } from './task-labels';

export type RecordContext = {
	caller: SupabaseClient;
	accountDirectory: SupabaseClient;
	requesterID: string;
	companyID: string;
	people: RecordPerson[];
	labels: CompanyLabels;
	leaveKinds: LeaveKind[];
	leaveYearStart: LeaveYearStart;
	locale: Locale;
	now: Date;
	decideTaskLabels: TaskLabelDecider;
};

type CompanyRow = {
	id: string;
	task_vocabulary: unknown;
	timezone: string | null;
	locale: string | null;
	rules: unknown;
};

export async function recordContextOf(
	caller: SupabaseClient,
	accountDirectory: SupabaseClient,
	requesterID: string,
	now: Date,
	decideTaskLabels: TaskLabelDecider
): Promise<RecordContext> {
	const company = await caller
		.from('company')
		.select('id, task_vocabulary, timezone, locale, rules')
		.limit(1)
		.single<CompanyRow>();
	if (company.error) throw new Error(company.error.message);

	return {
		caller,
		accountDirectory,
		requesterID,
		companyID: company.data.id,
		people: await peopleOfCompany(caller),
		labels: labelsOfVocabulary(company.data.task_vocabulary, company.data.timezone),
		leaveKinds: leaveKindsOfPolicy(company.data.rules),
		leaveYearStart: leaveYearStartOfPolicy(company.data.rules),
		locale: localeOf(company.data.locale),
		now,
		decideTaskLabels
	};
}
