import type { SupabaseClient } from '@supabase/supabase-js';
import { labelsOfVocabulary, type CompanyLabels } from './labels';
import { peopleOfCompany, type RecordPerson } from './people';

export type RecordContext = {
	caller: SupabaseClient;
	requesterID: string;
	people: RecordPerson[];
	labels: CompanyLabels;
	now: Date;
};

export async function recordContextOf(
	caller: SupabaseClient,
	requesterID: string,
	now: Date
): Promise<RecordContext> {
	const company = await caller
		.from('company')
		.select('task_vocabulary, timezone')
		.limit(1)
		.single<{ task_vocabulary: unknown; timezone: string | null }>();
	if (company.error) throw new Error(company.error.message);

	return {
		caller,
		requesterID,
		people: await peopleOfCompany(caller),
		labels: labelsOfVocabulary(company.data.task_vocabulary, company.data.timezone),
		now
	};
}
