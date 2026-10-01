import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { CompanySignupError, companySignupRequest, requestCompanySignupEmail } from '../../../src/lib/server/company-signup';
import { claimCodeLength } from '../../../src/routes/auth/claim/claim-code';

function isRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === 'object' && value !== null;
}

function recordOf(value: unknown): Record<string, unknown> {
	if (!isRecord(value)) throw new Error('expected a TOML table');
	return value;
}

describe('company sign-up email', () => {
	test('requires exactly one valid email field', () => {
		expect(companySignupRequest.safeParse({ email: 'founder@example.com' }).success).toBe(true);
		expect(companySignupRequest.safeParse({ email: 'founder@example.com', extra: true }).success).toBe(false);
		expect(companySignupRequest.safeParse({ email: 'not-an-email' }).success).toBe(false);
	});

	test('sends a Supabase OTP that creates the founder account', async () => {
		let received: { email: string; redirectURL: string; shouldCreateUser: boolean } | undefined;
		await requestCompanySignupEmail(
			{ SUPABASE_URL: 'https://project.supabase.co', SUPABASE_PUBLISHABLE_KEY: 'publishable' },
			'founder@example.com',
			'https://intern.example.com',
			async (request) => {
				received = {
					email: request.email,
					redirectURL: request.options.emailRedirectTo,
					shouldCreateUser: request.options.shouldCreateUser
				};
				return { error: null };
			}
		);

		expect(received).toEqual({
			email: 'founder@example.com',
			redirectURL: 'https://intern.example.com/auth/claim?new-company=1',
			shouldCreateUser: true
		});
	});

	test('keeps the hosted email template aligned with the claim code', () => {
		const configuration = recordOf(Bun.TOML.parse(readFileSync(new URL('../../../../supabase/config.toml', import.meta.url), 'utf8')));
		const email = recordOf(recordOf(configuration.auth).email);
		const templates = recordOf(email.template);
		const magicLink = recordOf(templates.magic_link);
		const confirmation = recordOf(templates.confirmation);
		const template = readFileSync(new URL('../../../../supabase/templates/magic-link.html', import.meta.url), 'utf8');

		expect(email.otp_length).toBe(claimCodeLength);
		expect(confirmation.content_path).toBe(magicLink.content_path);
		expect(template).toContain('{{ .Token }}');
	});

	test('requires the hosted Supabase settings before contacting auth', async () => {
		let wasCalled = false;
		const attempt = requestCompanySignupEmail({}, 'founder@example.com', 'https://intern.example.com', async () => {
			wasCalled = true;
			return { error: null };
		});

		await expect(attempt).rejects.toEqual(new CompanySignupError(500, 'the control plane is not configured'));
		expect(wasCalled).toBe(false);
	});

	test('preserves the provider rate-limit status and message', async () => {
		const attempt = requestCompanySignupEmail(
			{ SUPABASE_URL: 'https://project.supabase.co', SUPABASE_PUBLISHABLE_KEY: 'publishable' },
			'founder@example.com',
			'https://intern.example.com',
			async () => ({ error: { status: 429, message: 'rate limit reached' } })
		);

		await expect(attempt).rejects.toEqual(new CompanySignupError(429, 'rate limit reached'));
	});
});
