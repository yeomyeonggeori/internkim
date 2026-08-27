import { describe, expect, test } from 'bun:test';
import { fleetDirectory } from '../../../src/lib/server/fleet-directory';

async function refusalOf(call: Promise<unknown>): Promise<{ status: number; message: string }> {
	try {
		await call;
	} catch (thrown) {
		const refusal = thrown as { status?: number; body?: { message?: string } };
		return { status: refusal.status ?? 0, message: refusal.body?.message ?? '' };
	}
	throw new Error('expected a refusal');
}

describe('what an unconfigured plane says', () => {
	test('a plane with no project named blames itself, never the fleet', async () => {
		const refusal = await refusalOf(fleetDirectory({ SUPABASE_SECRET_KEY: 'secret' }, 'somefleet'));
		expect(refusal.status).toBe(500);
		expect(refusal.message).toBe('the central plane is not configured');
	});

	test('a plane with no key named says the same', async () => {
		const refusal = await refusalOf(
			fleetDirectory({ SUPABASE_URL: 'https://plane.supabase.co' }, 'somefleet')
		);
		expect(refusal.status).toBe(500);
		expect(refusal.message).toBe('the central plane is not configured');
	});
});
