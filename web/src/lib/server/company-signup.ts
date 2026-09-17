import { createClient } from '@supabase/supabase-js';
import { z } from 'zod';
import { mailCanCarryTheCode } from './claim-without-mail';

export type CompanySignupEnvironment = Record<string, string | undefined>;

type SignupError = { message: string; status?: number };
export const companySignupRequest = z.object({ email: z.string().trim().toLowerCase().email() }).strict();

export type CompanySignupOTPRequest = {
	email: string;
	options: { shouldCreateUser: true; emailRedirectTo: string };
};
type SendSignupEmail = (request: CompanySignupOTPRequest) => Promise<{ error: SignupError | null }>;

export class CompanySignupError extends Error {
	readonly status: number;

	constructor(status: number, message: string) {
		super(message);
		this.status = status;
		this.name = 'CompanySignupError';
	}
}

export async function requestCompanySignupEmail(
	environment: CompanySignupEnvironment,
	email: string,
	origin: string,
	sendEmail: SendSignupEmail = defaultSendSignupEmail(environment)
): Promise<void> {
	const projectURL = environment.SUPABASE_URL ?? '';
	const publishableKey = environment.SUPABASE_PUBLISHABLE_KEY ?? '';
	if (!projectURL || !publishableKey) throw new CompanySignupError(500, 'the control plane is not configured');
	if (!mailCanCarryTheCode(environment)) throw new CompanySignupError(503, 'company sign-up email is unavailable');

	const response = await sendEmail({
		email,
		options: { shouldCreateUser: true, emailRedirectTo: `${origin}/auth/claim?new-company=1` }
	});
	if (!response.error) return;
	const status = response.error.status && response.error.status >= 400 && response.error.status < 600
		? response.error.status
		: 502;
	throw new CompanySignupError(status, response.error.message);
}

function defaultSendSignupEmail(environment: CompanySignupEnvironment): SendSignupEmail {
	const projectURL = environment.SUPABASE_URL ?? '';
	const publishableKey = environment.SUPABASE_PUBLISHABLE_KEY ?? '';
	return async (request) => {
		const { error } = await createClient(projectURL, publishableKey).auth.signInWithOtp({
			email: request.email,
			options: request.options
		});
		return { error };
	};
}
