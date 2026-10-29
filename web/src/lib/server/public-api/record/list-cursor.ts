import { RecordRefusedTheWrite } from './tasks';

export function cursorOfPosition(position: string[]): string {
	const bytes = new TextEncoder().encode(JSON.stringify(position));
	return btoa(String.fromCharCode(...bytes));
}

export function positionOfCursor(cursor: string, toolName: string, length: number): string[] {
	const decoded = decodedCursor(cursor);
	if (!isPosition(decoded, length)) {
		throw new RecordRefusedTheWrite(`cursor is not one a ${toolName} result gave; omit it to start from the first page`, 400);
	}
	return decoded;
}

function isPosition(value: unknown, length: number): value is string[] {
	return Array.isArray(value) && value.length === length && value.every((part) => typeof part === 'string');
}

function decodedCursor(cursor: string): unknown {
	try {
		const bytes = Uint8Array.from(atob(cursor), (character) => character.charCodeAt(0));
		return JSON.parse(new TextDecoder('utf-8', { fatal: true }).decode(bytes));
	} catch {
		return undefined;
	}
}
