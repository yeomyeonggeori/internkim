import type { SupabaseClient } from '@supabase/supabase-js';
import { error } from '@sveltejs/kit';
import { permissionForTool, type ToolDescriptor } from '$lib/server/public-api/catalog';
import type { ToolAnswer } from '$lib/server/public-api/record';

const rememberedWrites = 'idempotency_key';

type RememberedWrite = { tool_name: string; response: ToolAnswer };

export function idempotencyKeyOffered(payload: Record<string, unknown>): string | null {
	const offered = payload.idempotencyKey;
	if (typeof offered !== 'string' || offered.trim() === '') return null;
	return offered.trim();
}

export function keyRemembersTheWrite(descriptor: ToolDescriptor, key: string | null): key is string {
	return key !== null && descriptor.idempotency.supported && permissionForTool(descriptor) !== 'read';
}

export async function writeRememberedFor(
	caller: SupabaseClient,
	memberID: string,
	key: string
): Promise<RememberedWrite | null> {
	const remembered = await caller
		.from(rememberedWrites)
		.select('tool_name, response')
		.eq('member_id', memberID)
		.eq('key', key)
		.maybeSingle<RememberedWrite>();
	if (remembered.error) error(500, remembered.error.message);
	return remembered.data;
}

export function answerRemembered(remembered: RememberedWrite, name: string): ToolAnswer {
	if (remembered.tool_name !== name) {
		return {
			status: 409,
			body: { error: `this idempotency key already answered ${remembered.tool_name}, and ${name} is another tool` }
		};
	}
	return remembered.response;
}

export async function rememberTheWrite(
	caller: SupabaseClient,
	memberID: string,
	key: string,
	name: string,
	answer: ToolAnswer
): Promise<void> {
	if (answer.status >= 300) return;
	const kept = await caller
		.from(rememberedWrites)
		.upsert(
			{ member_id: memberID, key, tool_name: name, response: answer },
			{ onConflict: 'member_id,key', ignoreDuplicates: true }
		);
	if (kept.error) {
		console.error(`tool.answer_not_remembered: tool=${name} key=${key} ${kept.error.message}`);
	}
}
