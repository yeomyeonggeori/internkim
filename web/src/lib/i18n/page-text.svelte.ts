import { currentLocale, type Locale } from './locale.svelte';

type TextTree = { readonly [key: string]: unknown };
type PageMessages = { readonly [key in Locale]: TextTree };

export function createPageText<const Messages extends PageMessages>(messages: Messages): Messages['ko'] {
	return createTextProxy(messages, []) as Messages['ko'];
}

function createTextProxy(messages: PageMessages, path: string[]): unknown {
	return new Proxy(
		{},
		{
			get(_target, property) {
				if (typeof property !== 'string') return undefined;

				const nextPath = [...path, property];
				const localizedValue = readPath(messages[currentLocale.value], nextPath);
				const fallbackValue = readPath(messages.ko, nextPath);
				const value = localizedValue ?? fallbackValue;

				if (isTextTree(value)) return createTextProxy(messages, nextPath);
				if (typeof value === 'string') return value;
				return '';
			}
		}
	);
}

function readPath(root: unknown, path: string[]) {
	let value = root;
	for (const segment of path) {
		if (!isTextTree(value)) return undefined;
		value = value[segment];
	}
	return value;
}

function isTextTree(value: unknown): value is TextTree {
	return typeof value === 'object' && value !== null && !Array.isArray(value);
}
