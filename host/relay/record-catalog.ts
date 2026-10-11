export type MemberSession = { accessToken: string; expiresAt: number };

export type RecordCatalogSettings = {
	appURL: string;
	loopbackURL: string;
	mintFor: (requesterEmail: string) => Promise<MemberSession>;
	report?: (line: string) => void;
};

export type McpServerEntry = {
	type: 'http';
	name: string;
	url: string;
	headers: { name: string; value: string }[];
};

export const recordCatalogPath = '/mcp/';
export const recordCatalogName = 'internkim';

/**
 * A member session expires after `jwt_expiry` in supabase/config.toml, an hour,
 * and the five-minute margin below is the one internal/centralplane/client.go
 * keeps against the same endpoint.
 */
export class RecordCatalogs {
	private readonly settings: RecordCatalogSettings;
	private readonly emailByTicket = new Map<string, string>();
	private readonly sessionByEmail = new Map<string, MemberSession>();
	private readonly mintingByEmail = new Map<string, Promise<MemberSession>>();

	constructor(settings: RecordCatalogSettings) {
		this.settings = settings;
	}

	serversFor(requesterEmail: string, ticket: string): McpServerEntry[] {
		const email = requesterEmail.trim().toLowerCase();
		if (!email) return [];
		this.emailByTicket.set(ticket, email);
		return [
			{
				type: 'http',
				name: recordCatalogName,
				url: `${this.settings.loopbackURL}${recordCatalogPath}${ticket}`,
				headers: []
			}
		];
	}

	async serve(request: Request, ticket: string): Promise<Response> {
		const call = await request.text();
		const email = this.emailByTicket.get(ticket);
		if (!email) return refusedCall(call, 'this relay opened no record catalog under that address');
		let session: MemberSession;
		try {
			session = await this.sessionFor(email);
		} catch (failure) {
			this.settings.report?.(`no session for ${email}: ${String(failure)}`);
			return refusedCall(call, `the record would not open a session for ${email}: ${String(failure)}`);
		}
		const answered = await this.forward(request, call, session).catch((failure: unknown) => failure);
		if (answered instanceof Response) return answeredOrRefused(call, answered);
		this.settings.report?.(`the company app did not answer: ${String(answered)}`);
		return refusedCall(call, `the company app at ${this.settings.appURL} could not be reached: ${String(answered)}`);
	}

	private forward(request: Request, call: string, session: MemberSession): Promise<Response> {
		return fetch(`${this.settings.appURL}/api/v1/mcp`, {
			method: 'POST',
			headers: {
				Authorization: `Bearer ${session.accessToken}`,
				'Content-Type': request.headers.get('content-type') ?? 'application/json',
				Accept: request.headers.get('accept') ?? 'application/json'
			},
			body: call
		});
	}

	private async sessionFor(requesterEmail: string): Promise<MemberSession> {
		const held = this.sessionByEmail.get(requesterEmail);
		if (held && held.expiresAt - Math.floor(Date.now() / 1000) > 300) return held;
		const alreadyMinting = this.mintingByEmail.get(requesterEmail);
		if (alreadyMinting) return alreadyMinting;
		const minting = this.settings
			.mintFor(requesterEmail)
			.then((session) => {
				this.sessionByEmail.set(requesterEmail, session);
				return session;
			})
			.finally(() => this.mintingByEmail.delete(requesterEmail));
		this.mintingByEmail.set(requesterEmail, minting);
		return minting;
	}
}

export function ticketOf(pathname: string): string {
	if (!pathname.startsWith(recordCatalogPath)) return '';
	return pathname.slice(recordCatalogPath.length);
}

async function answeredOrRefused(call: string, answered: Response): Promise<Response> {
	const contentType = answered.headers.get('content-type') ?? 'application/json';
	if (answered.ok) return new Response(answered.body, { status: answered.status, headers: { 'Content-Type': contentType } });
	const said = (await answered.text()).trim();
	return refusedCall(call, `the company app answered ${answered.status}${said ? `: ${said}` : ''}`);
}

// JSON-RPC 2.0 §5.1: an error for a request is a response object carrying the
// request's id.
const failedCallCode = -32603;

function refusedCall(call: string, reason: string): Response {
	return Response.json({ jsonrpc: '2.0', id: requestIDIn(call), error: { code: failedCallCode, message: reason } });
}

function requestIDIn(call: string): string | number | null {
	try {
		const id: unknown = JSON.parse(call)?.id;
		return typeof id === 'string' || typeof id === 'number' ? id : null;
	} catch {
		return null;
	}
}
