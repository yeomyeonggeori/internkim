import { randomUUID } from 'node:crypto';

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
	private readonly ticketByConversation = new Map<string, string>();
	private readonly emailByTicket = new Map<string, string>();
	private readonly sessionByEmail = new Map<string, MemberSession>();
	private readonly mintingByEmail = new Map<string, Promise<MemberSession>>();

	constructor(settings: RecordCatalogSettings) {
		this.settings = settings;
	}

	serversFor(requesterEmail: string, conversationID: string): McpServerEntry[] {
		const email = requesterEmail.trim().toLowerCase();
		if (!email) return [];
		const ticket = this.ticketFor(email, conversationID);
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
		const email = this.emailByTicket.get(ticket);
		if (!email) return new Response('no catalog was opened under that name', { status: 404 });
		let session: MemberSession;
		try {
			session = await this.sessionFor(email);
		} catch (failure) {
			this.settings.report?.(`no session for ${email}: ${String(failure)}`);
			return new Response('the record would not open a session for that person', { status: 502 });
		}
		const answered = await fetch(`${this.settings.appURL}/api/v1/mcp`, {
			method: 'POST',
			headers: {
				Authorization: `Bearer ${session.accessToken}`,
				'Content-Type': request.headers.get('content-type') ?? 'application/json',
				Accept: request.headers.get('accept') ?? 'application/json'
			},
			body: await request.text()
		});
		return new Response(answered.body, {
			status: answered.status,
			headers: { 'Content-Type': answered.headers.get('content-type') ?? 'application/json' }
		});
	}

	private ticketFor(requesterEmail: string, conversationID: string): string {
		const held = this.ticketByConversation.get(conversationID);
		if (held) {
			this.emailByTicket.set(held, requesterEmail);
			return held;
		}
		const ticket = randomUUID();
		this.ticketByConversation.set(conversationID, ticket);
		this.emailByTicket.set(ticket, requesterEmail);
		return ticket;
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
