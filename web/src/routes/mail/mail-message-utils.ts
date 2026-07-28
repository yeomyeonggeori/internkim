import type { MailMessage } from './mail-types';

export const MAIL_MESSAGE_IFRAME_SANDBOX = 'allow-popups allow-popups-to-escape-sandbox';

export function mergeMailMessages(existingMessages: MailMessage[], incomingMessages: MailMessage[]) {
	const seenMessages = new Set(existingMessages.map((message) => mailMessageKey(message)));
	const mergedMessages = [...existingMessages];
	for (const message of incomingMessages) {
		const key = mailMessageKey(message);
		if (seenMessages.has(key)) continue;
		seenMessages.add(key);
		mergedMessages.push(message);
	}
	return mergedMessages;
}

export function mailMessageKey(message: MailMessage) {
	return `${message.mailbox}:${message.uid}`;
}

export function mailMessageTimeLabel(date: string, yesterdayLabel: string, now = new Date(), locale?: string) {
	const receivedAt = new Date(date);
	if (!date || Number.isNaN(receivedAt.getTime())) return '';
	const startOfToday = new Date(now.getFullYear(), now.getMonth(), now.getDate());
	if (receivedAt >= startOfToday) return new Intl.DateTimeFormat(locale, { hour: 'numeric', minute: '2-digit' }).format(receivedAt);
	const startOfYesterday = new Date(startOfToday.getFullYear(), startOfToday.getMonth(), startOfToday.getDate() - 1);
	if (receivedAt >= startOfYesterday) return yesterdayLabel;
	const isSameYear = receivedAt.getFullYear() === now.getFullYear();
	return new Intl.DateTimeFormat(locale, isSameYear ? { month: 'numeric', day: 'numeric' } : { year: 'numeric', month: 'numeric', day: 'numeric' }).format(receivedAt);
}

export function mailHTMLDocument(bodyHTML: string) {
	return `<!doctype html>
<html>
<head>
	<meta charset="utf-8">
	<meta name="referrer" content="no-referrer">
	<meta http-equiv="Content-Security-Policy" content="default-src 'none'; img-src https: http: data: cid:; font-src https: http: data:; style-src 'unsafe-inline' https: http: data:; script-src 'none'; object-src 'none'; base-uri 'none'; form-action 'none'">
	<style>
		html, body { margin: 0; padding: 0; background: #ffffff; color: #111827; font: 14px/1.6 -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; }
		body { overflow-wrap: anywhere; }
		img, table { max-width: 100%; }
		a { color: #0f4c81; }
	</style>
</head>
<body>${mailHTMLWithSafeLinkTargets(bodyHTML)}</body>
</html>`;
}

function mailHTMLWithSafeLinkTargets(bodyHTML: string) {
	return bodyHTML.replace(/<a\b([^>]*)>/gi, (_anchor, attributes: string) => {
		const nextAttributes = mailHTMLAnchorAttributesWithSafeRel(mailHTMLAnchorAttributesWithBlankTarget(attributes));
		return `<a${nextAttributes}>`;
	});
}

function mailHTMLAnchorAttributesWithBlankTarget(attributes: string) {
	if (mailHTMLAnchorHasAttribute(attributes, 'target')) {
		return attributes.replace(/\starget\s*=\s*(?:"[^"]*"|'[^']*'|[^\s>]+)/i, ' target="_blank"');
	}
	return ` target="_blank"${attributes}`;
}

function mailHTMLAnchorAttributesWithSafeRel(attributes: string) {
	if (!mailHTMLAnchorHasAttribute(attributes, 'rel')) return ` rel="noopener noreferrer"${attributes}`;
	return attributes.replace(/\srel\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+))/i, (_match, doubleQuotedValue: string | undefined, singleQuotedValue: string | undefined, unquotedValue: string | undefined) => {
		const relValue = doubleQuotedValue ?? singleQuotedValue ?? unquotedValue ?? '';
		const relTokens = new Set(relValue.split(/\s+/).filter(Boolean));
		relTokens.add('noopener');
		relTokens.add('noreferrer');
		return ` rel="${[...relTokens].join(' ')}"`;
	});
}

function mailHTMLAnchorHasAttribute(attributes: string, name: string) {
	return new RegExp(`\\s${name}\\s*=`, 'i').test(` ${attributes}`);
}
