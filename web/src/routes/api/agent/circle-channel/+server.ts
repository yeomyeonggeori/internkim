import { error, json } from '@sveltejs/kit';
import { z } from 'zod';
import type { RequestHandler } from './$types';
import { callingAgent, environmentOf } from '$lib/server/agent-request';

type ChannelledCircleRow = {
	id: string;
	channel_name: string | null;
	channel_id: string | null;
	circle_member: { member: { email: string; status: string } | null }[];
};

const realizedChannelSchema = z.strictObject({
	circleID: z.string().trim().min(1),
	channelID: z.string().trim().min(1).nullable()
});

export const GET: RequestHandler = async ({ request, platform }) => {
	const { client, companyID } = await callingAgent(request, environmentOf(platform));

	const channelled = await client
		.from('circle')
		.select('id, channel_name, channel_id, circle_member(member(email, status))')
		.eq('company_id', companyID)
		.or('channel_name.not.is.null,channel_id.not.is.null')
		.order('id')
		.returns<ChannelledCircleRow[]>();
	if (channelled.error) return json({ error: channelled.error.message }, { status: 502 });

	return json({ circles: (channelled.data ?? []).map(circleChannelOf) });
};

export const PUT: RequestHandler = async ({ request, platform }) => {
	const { client, companyID } = await callingAgent(request, environmentOf(platform));

	const realized = realizedChannelSchema.safeParse(await request.json().catch(() => ({})));
	if (!realized.success) error(400, 'a realized circle channel names the circle and the channel opened for it, or null');

	const recorded = await client
		.from('circle')
		.update({ channel_id: realized.data.channelID })
		.eq('company_id', companyID)
		.eq('id', realized.data.circleID)
		.select('id');
	if (recorded.error) return json({ error: recorded.error.message }, { status: 502 });
	if ((recorded.data ?? []).length === 0) error(404, `no circle here goes by ${realized.data.circleID}`);
	return json({ recorded: true });
};

function circleChannelOf(circle: ChannelledCircleRow) {
	return {
		circleID: circle.id,
		channelName: circle.channel_name,
		channelID: circle.channel_id,
		memberEmails: circle.circle_member
			.flatMap(({ member }) => (member?.status === 'active' ? [member.email.toLowerCase()] : []))
			.sort()
	};
}
