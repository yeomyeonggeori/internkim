import { currentLocale, type Locale } from './locale.svelte';

type TextTree = { readonly [key: string]: unknown };
type PageMessages = { readonly [key in Locale]: TextTree };

type LocalizedValue<Value> = Value extends string
	? string
	: Value extends readonly (infer Item)[]
		? readonly LocalizedValue<Item>[]
		: { [Key in keyof Value]: LocalizedValue<Value[Key]> };

export type PageText<Messages extends PageMessages> = {
	[Key in keyof Messages['ko']]: LocalizedValue<Messages['ko'][Key]>;
};

export function createPageText<const Messages extends PageMessages>(
	messages: Messages
): PageText<Messages> {
	return createTextProxy(messages, []) as PageText<Messages>;
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
				if (Array.isArray(value)) return value;
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
