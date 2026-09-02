import type { SupabaseClient } from '@supabase/supabase-js';
import { describe, expect, test } from 'bun:test';
import { changeOwnPassword, WrongCurrentPasswordError } from '../../src/lib/account-password';

type AuthAnswer = { error: { message: string; status?: number } | null };

const signedIn = { data: { session: { user: { email: 'sample@example.com' } } } };

function clientThat(options: {
	session?: unknown;
	signIn?: AuthAnswer;
	update?: AuthAnswer;
	proven?: { email: string; password: string }[];
	updated?: string[];
}): SupabaseClient {
	return {
		auth: {
			getSession: async () => options.session ?? signedIn,
			signInWithPassword: async (credentials: { email: string; password: string }) => {
				options.proven?.push(credentials);
				return options.signIn ?? { error: null };
			},
			updateUser: async (attributes: { password: string }) => {
				options.updated?.push(attributes.password);
				return options.update ?? { error: null };
			}
		}
	} as unknown as SupabaseClient;
}

describe('changing your own password', () => {
	test('proves the current password before setting the new one', async () => {
		const proven: { email: string; password: string }[] = [];
		const updated: string[] = [];

		await changeOwnPassword('old-one', 'new-one', clientThat({ proven, updated }));

		expect(proven).toEqual([{ email: 'sample@example.com', password: 'old-one' }]);
		expect(updated).toEqual(['new-one']);
	});

	test('leaves the password alone when the current one is wrong', async () => {
		const updated: string[] = [];
		const client = clientThat({
			signIn: { error: { message: 'Invalid login credentials', status: 400 } },
			updated
		});

		await expect(changeOwnPassword('guessed', 'new-one', client)).rejects.toBeInstanceOf(
			WrongCurrentPasswordError
		);
		expect(updated).toEqual([]);
	});

	test('reports a sign-in failure that is not a wrong password as itself', async () => {
		const client = clientThat({ signIn: { error: { message: 'the network is down', status: 503 } } });

		await expect(changeOwnPassword('old-one', 'new-one', client)).rejects.toThrow('the network is down');
	});

	test('refuses when nobody is signed in', async () => {
		const client = clientThat({ session: { data: { session: null } } });

		await expect(changeOwnPassword('old-one', 'new-one', client)).rejects.toThrow(
			'no signed-in account to change a password for'
		);
	});

	test('carries the update failure back', async () => {
		const client = clientThat({ update: { error: { message: 'Password should be at least 6 characters' } } });

		await expect(changeOwnPassword('old-one', 'short', client)).rejects.toThrow(
			'Password should be at least 6 characters'
		);
	});
});
