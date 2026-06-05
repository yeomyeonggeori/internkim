import type { MailMessage } from './mail-types';

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

export function mailHTMLDocument(bodyHTML: string) {
	return `<!doctype html>
<html>
<head>
	<meta charset="utf-8">
	<meta name="referrer" content="no-referrer">
	<meta http-equiv="Content-Security-Policy" content="default-src 'none'; img-src data: cid:; font-src https: http: data:; style-src 'unsafe-inline' https: http: data:; script-src 'none'; object-src 'none'; base-uri 'none'; form-action 'none'">
	<style>
		html, body { margin: 0; padding: 0; background: #ffffff; color: #111827; font: 14px/1.6 -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; }
		body { overflow-wrap: anywhere; }
		img, table { max-width: 100%; }
		a { color: #0f4c81; }
	</style>
</head>
<body>${bodyHTML}</body>
</html>`;
}
