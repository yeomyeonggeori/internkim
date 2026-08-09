import { json } from '@sveltejs/kit';
import { environmentOf } from '$lib/server/agent-request';
import { callingMember } from '$lib/server/member-request';
import {
	asShown,
	asWritten,
	keepMailAccount,
	mailAccountOfMember,
	type MailAccountAsWritten
} from '$lib/server/mail-account';
import type { RequestHandler } from './$types';

export const GET: RequestHandler = async ({ request, platform }) => {
	const { record, memberID } = await callingMember(request, environmentOf(platform));
	return json({ account: asShown(await mailAccountOfMember(record, memberID)) });
};

export const PUT: RequestHandler = async ({ request, platform }) => {
	const { record, memberID, email } = await callingMember(request, environmentOf(platform));

	const written = (await request.json().catch(() => ({}))) as MailAccountAsWritten;
	const held = await mailAccountOfMember(record, memberID);
	const account = asWritten(written, email, held);
	await keepMailAccount(record, memberID, account);

	return json({ account: asShown(account) });
};
