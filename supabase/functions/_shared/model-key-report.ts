import type { CallingComputer } from './agent-caller.ts';
import { json, refuse } from './http.ts';
import type { SupabaseClient } from './service-client.ts';

export async function keptModelKeyReport(
	client: SupabaseClient,
	companyID: string,
	computer: CallingComputer,
	report: Record<string, unknown>
): Promise<Response> {
	if (typeof report.holdsModelKey !== 'boolean') refuse(400, 'say whether this computer holds an OpenRouter key, as holdsModelKey');
	const { data, error } = await client.rpc('keep_model_key_report', {
		reporting_company: companyID,
		reporting_box: computer.boxKey,
		holds: report.holdsModelKey
	});
	if (error) refuse(502, `the record could not keep what this computer said about its key: ${error.message}`);
	if (typeof data !== 'boolean') refuse(502, 'the record holds no answer to whether the company uses its own key');
	return json({ usesOwnModelKey: data });
}
