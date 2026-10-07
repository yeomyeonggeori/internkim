import type { ChannelMessage } from '$lib/components/channel/channel-api';
import { clockTime, dateKeyOf, dateLabel } from '$lib/components/channel/channel-time';

export type ExportFormat = 'text' | 'csv' | 'pdf';

export type ExportLabels = {
	savedAt: string;
	attachment: string;
	date: string;
	time: string;
	sender: string;
	content: string;
	attachments: string;
};

export type ConversationDocument = {
	title: string;
	savedAt: string;
	messages: ChannelMessage[];
	labels: ExportLabels;
};

type DayOfMessages = { label: string; messages: ChannelMessage[] };

const byteOrderMark = '﻿';
const csvLineBreak = '\r\n';
const dayRule = '-'.repeat(12);
const spreadsheetFormulaStart = /^[=+\-@\t\r]/;

function daysOf(messages: ChannelMessage[]): DayOfMessages[] {
	const days: DayOfMessages[] = [];
	let currentKey = '';
	for (const message of messages) {
		const key = dateKeyOf(message.sentAt);
		if (key !== currentKey) {
			days.push({ label: dateLabel(message.sentAt), messages: [] });
			currentKey = key;
		}
		days[days.length - 1].messages.push(message);
	}
	return days;
}

function attachmentNamesOf(message: ChannelMessage): string[] {
	return (message.attachments ?? []).map((attachment) => attachment.filename || attachment.url);
}

function savedAtLine(document: ConversationDocument): string {
	return `${document.labels.savedAt}: ${dateLabel(document.savedAt)} ${clockTime(document.savedAt)}`;
}

export function conversationAsText(document: ConversationDocument): string {
	const lines = [document.title, savedAtLine(document)];
	for (const day of daysOf(document.messages)) {
		lines.push('', `${dayRule} ${day.label} ${dayRule}`);
		for (const message of day.messages) {
			lines.push(`[${message.sender.name}] [${clockTime(message.sentAt)}] ${message.text}`);
			for (const name of attachmentNamesOf(message)) lines.push(`${document.labels.attachment}: ${name}`);
		}
	}
	return `${lines.join('\n')}\n`;
}

function csvCell(value: string): string {
	const safe = spreadsheetFormulaStart.test(value) ? `'${value}` : value;
	return /[",\r\n]/.test(safe) ? `"${safe.replaceAll('"', '""')}"` : safe;
}

export function conversationAsCSV(document: ConversationDocument): string {
	const { labels } = document;
	const header = [labels.date, labels.time, labels.sender, labels.content, labels.attachments];
	const rows = document.messages.map((message) => [
		dateKeyOf(message.sentAt),
		clockTime(message.sentAt),
		message.sender.name,
		message.text,
		attachmentNamesOf(message).join('; ')
	]);
	const lines = [header, ...rows].map((row) => row.map(csvCell).join(','));
	return `${byteOrderMark}${lines.join(csvLineBreak)}${csvLineBreak}`;
}

function escapedHTML(value: string): string {
	return value
		.replaceAll('&', '&amp;')
		.replaceAll('<', '&lt;')
		.replaceAll('>', '&gt;')
		.replaceAll('"', '&quot;')
		.replaceAll("'", '&#39;');
}

const printStyle = `
body { font-family: system-ui, sans-serif; font-size: 12px; color: #111; margin: 24px; }
h1 { font-size: 18px; margin: 0 0 4px; }
.saved { color: #666; margin: 0 0 16px; }
h2 { font-size: 12px; color: #666; font-weight: 500; text-align: center; border-top: 1px solid #ddd; padding-top: 8px; margin: 16px 0 8px; }
.message { margin: 0 0 8px; break-inside: avoid; }
.meta { font-weight: 600; }
.meta time { color: #666; font-weight: 400; margin-left: 6px; }
.body { white-space: pre-wrap; overflow-wrap: anywhere; }
.file { color: #444; }
`;

export function conversationAsPrintableHTML(document: ConversationDocument): string {
	const days = daysOf(document.messages).map((day) => {
		const messages = day.messages.map((message) => {
			const files = attachmentNamesOf(message)
				.map((name) => `<div class="file">${escapedHTML(`${document.labels.attachment}: ${name}`)}</div>`)
				.join('');
			return `<div class="message"><div class="meta">${escapedHTML(message.sender.name)}<time>${escapedHTML(clockTime(message.sentAt))}</time></div><div class="body">${escapedHTML(message.text)}</div>${files}</div>`;
		});
		return `<h2>${escapedHTML(day.label)}</h2>${messages.join('')}`;
	});
	return `<!doctype html><html><head><meta charset="utf-8"><title>${escapedHTML(document.title)}</title><style>${printStyle}</style></head><body><h1>${escapedHTML(document.title)}</h1><p class="saved">${escapedHTML(savedAtLine(document))}</p>${days.join('')}</body></html>`;
}

export function exportFilename(title: string, savedAt: string, extension: 'txt' | 'csv'): string {
	const safeTitle = title.replace(/[\\/:*?"<>|\u0000-\u001f]/g, '_').trim() || 'conversation';
	return `${safeTitle}-${dateKeyOf(savedAt)}.${extension}`;
}
