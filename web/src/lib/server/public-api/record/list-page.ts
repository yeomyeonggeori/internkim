import { listPageSize } from '../catalog/list-page';
import { RecordRefusedTheWrite } from './tasks';

export type ListPageInput = { limit?: number; cursor?: string };

export type ListPage<Item> = { items: Item[]; totalCount: number; nextCursor?: string };

export type ListOrder<Item> = {
	positionOf: (item: Item) => string[];
	compare: (left: string[], right: string[]) => number;
};

export function ascendingPositions(left: string[], right: string[]): number {
	for (let index = 0; index < Math.max(left.length, right.length); index++) {
		const leftPart = left[index] ?? '';
		const rightPart = right[index] ?? '';
		if (leftPart !== rightPart) return leftPart < rightPart ? -1 : 1;
	}
	return 0;
}

export function descendingPositions(left: string[], right: string[]): number {
	return ascendingPositions(right, left);
}

export function pageOf<Item>(toolName: string, items: Item[], order: ListOrder<Item>, input: ListPageInput): ListPage<Item> {
	const ordered = [...items].sort((left, right) => order.compare(order.positionOf(left), order.positionOf(right)));
	const after = input.cursor === undefined ? undefined : positionOfCursor(toolName, input.cursor);
	const remaining = after ? ordered.filter((item) => order.compare(order.positionOf(item), after) > 0) : ordered;
	const page = remaining.slice(0, input.limit ?? listPageSize);
	const last = page.at(-1);
	return {
		items: page,
		totalCount: ordered.length,
		...(last && remaining.length > page.length ? { nextCursor: cursorOfPosition(order.positionOf(last)) } : {})
	};
}

function cursorOfPosition(position: string[]): string {
	const bytes = new TextEncoder().encode(JSON.stringify(position));
	return btoa(String.fromCharCode(...bytes));
}

function positionOfCursor(toolName: string, cursor: string): string[] {
	const decoded = decodedCursor(cursor);
	if (!isPosition(decoded)) {
		throw new RecordRefusedTheWrite(`cursor is not one a ${toolName} result gave; omit it to start from the first page`, 400);
	}
	return decoded;
}

function isPosition(value: unknown): value is string[] {
	return Array.isArray(value) && value.length > 0 && value.every((part) => typeof part === 'string');
}

function decodedCursor(cursor: string): unknown {
	try {
		const bytes = Uint8Array.from(atob(cursor), (character) => character.charCodeAt(0));
		return JSON.parse(new TextDecoder('utf-8', { fatal: true }).decode(bytes));
	} catch {
		return undefined;
	}
}
