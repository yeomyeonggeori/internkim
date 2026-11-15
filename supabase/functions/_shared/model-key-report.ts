import { json, refuse } from './http.ts';
import type { SupabaseClient } from './service-client.ts';

export async function keptModelKeyReport(client: SupabaseClient, companyID: string, report: Record<string, unknown>): Promise<Response> {
	if (typeof report.holdsModelKey !== 'boolean') refuse(400, 'say whether this computer holds an OpenRouter key, as holdsModelKey');
	const { data, error } = await client
		.from('company')
		.update({ host_holds_model_key: report.holdsModelKey })
		.eq('id', companyID)
		.select('uses_own_model_key')
		.single();
	if (error) refuse(502, `the record could not keep what this computer said about its key: ${error.message}`);
	if (typeof data.uses_own_model_key !== 'boolean') refuse(502, 'the record holds no answer to whether the company uses its own key');
	return json({ usesOwnModelKey: data.uses_own_model_key });
}
