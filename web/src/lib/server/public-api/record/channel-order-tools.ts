import { z } from 'zod';
import { RecordRefusedTheWrite, statusOfPostgresCode } from './tasks';
import type { RecordContext } from './company';
import {
	channelOrderResultSchema,
	channelOrderSetInputSchema,
	channelPinInputSchema,
	type ChannelOrderResult,
	type ChannelPinResult
} from '../catalog/channel-order';

export type ChannelOrderSetInput = z.infer<typeof channelOrderSetInputSchema>;
export type ChannelPinInput = z.infer<typeof channelPinInputSchema>;

function channelOrderAnswered(name: string, answered: unknown): ChannelOrderResult {
	const parsed = channelOrderResultSchema.safeParse(answered);
	if (!parsed.success) throw new Error(`${name} answered no channel order, so the requester has no member row to keep one on`);
	return parsed.data;
}

async function channelOrderWritten(
	context: RecordContext,
	name: string,
	parameters: Record<string, unknown>
): Promise<ChannelOrderResult> {
	const { data, error } = await context.caller.rpc(name, parameters);
	if (error) throw new RecordRefusedTheWrite(error.message, statusOfPostgresCode(error.code));
	return channelOrderAnswered(name, data);
}

export async function channelOrderGet(context: RecordContext): Promise<ChannelOrderResult> {
	const { data, error } = await context.caller.rpc('channel_order_get');
	if (error) throw new Error(error.message);
	return channelOrderAnswered('channel_order_get', data);
}

export async function channelOrderSet(context: RecordContext, input: ChannelOrderSetInput): Promise<ChannelOrderResult> {
	if (!input.pinnedChannels && !input.channelOrder) {
		throw new Error('an order change names the pinned channels, the channel order, or both');
	}
	return channelOrderWritten(context, 'channel_order_set', {
		pinned: input.pinnedChannels ?? null,
		ordered: input.channelOrder ?? null
	});
}

export async function channelPin(context: RecordContext, input: ChannelPinInput): Promise<ChannelPinResult> {
	const order = await channelOrderWritten(context, 'channel_pin', { channel: input.channelID });
	return { channelID: input.channelID, isPinned: true, ...order };
}

export async function channelUnpin(context: RecordContext, input: ChannelPinInput): Promise<ChannelPinResult> {
	const order = await channelOrderWritten(context, 'channel_unpin', { channel: input.channelID });
	return { channelID: input.channelID, isPinned: false, ...order };
}
