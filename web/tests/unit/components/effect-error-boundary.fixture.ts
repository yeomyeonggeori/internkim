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
const { default: PageShell } = await import('./effect-error-boundary-fixtures/page-shell.svelte');

const reportedErrors: string[] = [];
console.error = (_message: string, error: unknown) => reportedErrors.push(String(error));

async function renderShell(isContained: boolean): Promise<ShellOutcome> {
	return render(() => mount(Shell, { target: newTarget(), props: { isContained } }));
}

async function renderPageShell(): Promise<ShellOutcome> {
	return render(() => mount(PageShell, { target: newTarget() }));
}

async function renderPageShellThenLeave(): Promise<ShellOutcome> {
	return render(() => mount(PageShell, { target: newTarget() }), leaveThePage);
}

function leaveThePage(): void {
	document.querySelector<HTMLButtonElement>('[data-leave]')?.click();
	flushSync();
}

interface ShellOutcome {
	failureText: string | null;
	pageText: string | null;
	gateText: string | null;
	reportedErrors: string[];
	thrown: string | null;
}

function newTarget(): HTMLElement {
	const target = document.createElement('div');
	document.body.appendChild(target);
	return target;
}

async function render(mountShell: () => ReturnType<typeof mount>, afterwards: () => void = () => {}): Promise<ShellOutcome> {
	reportedErrors.length = 0;
	let thrown: string | null = null;
	let component: ReturnType<typeof mount> | null = null;
	try {
		component = mountShell();
		flushSync();
	} catch (error) {
		thrown = String(error);
	}
	await new Promise((resolve) => setTimeout(resolve, 20));
	afterwards();
	await new Promise((resolve) => setTimeout(resolve, 20));
	const gateText = document.querySelector('[data-gate]')?.textContent ?? null;
	const failureText = document.querySelector('[data-failure]')?.textContent ?? null;
	const pageText = document.querySelector('[data-page]')?.textContent ?? null;
	const outcome = { failureText, pageText, gateText, reportedErrors: [...reportedErrors], thrown };
	if (component) unmount(component);
	document.body.replaceChildren();
	return outcome;
}

console.log(JSON.stringify({ contained: await renderShell(true), uncontained: await renderShell(false), page: await renderPageShell(), left: await renderPageShellThenLeave() }));
