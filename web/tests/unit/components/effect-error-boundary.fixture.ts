import { readFileSync } from 'node:fs';
import { compile } from 'svelte/compiler';
import { Window } from 'happy-dom';

const browserWindow = new Window();
for (const name of Object.getOwnPropertyNames(browserWindow)) {
	if (name in globalThis) continue;
	Object.defineProperty(globalThis, name, { value: Reflect.get(browserWindow, name), configurable: true, writable: true });
}
Object.assign(globalThis, { window: browserWindow, document: browserWindow.document });

Bun.plugin({
	name: 'svelte',
	setup(build) {
		build.onLoad({ filter: /\.svelte$/ }, ({ path }) => ({
			contents: compile(readFileSync(path, 'utf8'), { filename: path, generate: 'client' }).js.code,
			loader: 'js'
		}));
	}
});

const { flushSync, mount, unmount } = await import('svelte');
const { default: Shell } = await import('./effect-error-boundary-fixtures/shell.svelte');

const reportedErrors: string[] = [];
console.error = (_message: string, error: unknown) => reportedErrors.push(String(error));

async function renderShell(isContained: boolean): Promise<{ gateText: string | null; reportedErrors: string[] }> {
	reportedErrors.length = 0;
	const target = document.createElement('div');
	document.body.appendChild(target);
	let thrown: string | null = null;
	let component: ReturnType<typeof mount> | null = null;
	try {
		component = mount(Shell, { target, props: { isContained } });
		flushSync();
	} catch (error) {
		thrown = String(error);
	}
	await new Promise((resolve) => setTimeout(resolve, 20));
	const gateText = target.querySelector('[data-gate]')?.textContent ?? null;
	const outcome = { gateText, reportedErrors: [...reportedErrors], thrown };
	if (component) unmount(component);
	target.remove();
	return outcome;
}

console.log(JSON.stringify({ contained: await renderShell(true), uncontained: await renderShell(false) }));
