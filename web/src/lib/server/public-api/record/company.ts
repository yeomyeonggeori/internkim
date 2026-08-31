import type { SupabaseClient } from '@supabase/supabase-js';
import { labelsOfVocabulary, type CompanyLabels } from './labels';
import { leaveKindsOfPolicy, type LeaveKind } from './leave';
import { peopleOfCompany, type RecordPerson } from './people';

export type RecordContext = {
	caller: SupabaseClient;
	requesterID: string;
	people: RecordPerson[];
	labels: CompanyLabels;
	leaveKinds: LeaveKind[];
	now: Date;
};

export async function recordContextOf(
	caller: SupabaseClient,
	requesterID: string,
	now: Date
): Promise<RecordContext> {
	const company = await caller
		.from('company')
		.select('task_vocabulary, timezone, rules')
		.limit(1)
		.single<{ task_vocabulary: unknown; timezone: string | null; rules: unknown }>();
	if (company.error) throw new Error(company.error.message);

	return {
		caller,
		requesterID,
		people: await peopleOfCompany(caller),
		labels: labelsOfVocabulary(company.data.task_vocabulary, company.data.timezone),
		leaveKinds: leaveKindsOfPolicy(company.data.rules),
		now
	};
}
