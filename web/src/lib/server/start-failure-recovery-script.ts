import { moduleLoadErrorPhrases } from '$lib/module-load-recovery';

export const startFailureRecoveryPlaceholder = '<script id="start-failure-recovery"></script>';

const recovery = `(() => {
	const reloadedKey = 'internkim-reloaded-for-release';
	const phrases = ${JSON.stringify(moduleLoadErrorPhrases)};
	const root = document.documentElement;
	const hasStarted = () => 'hydrated' in root.dataset;
	const sayItFailed = () => {
		if (!document.body || document.getElementById('start-failure')) return;
		const notice = document.createElement('div');
		notice.id = 'start-failure';
		notice.setAttribute('role', 'alert');
		notice.style.cssText = 'position:fixed;inset:0;z-index:2147483646;display:grid;place-content:center;justify-items:center;gap:1rem;padding:1.5rem;text-align:center;font:1rem/1.5 system-ui,sans-serif;background:var(--background,Canvas);color:var(--foreground,CanvasText)';
		const said = document.createElement('p');
		said.style.margin = '0';
		said.append('앱을 불러오지 못했어요.', document.createElement('br'), 'The app could not load.');
		const again = document.createElement('button');
		again.type = 'button';
		again.textContent = '다시 시도 · Retry';
		again.style.cssText = 'padding:0.5rem 1rem;border:1px solid currentColor;border-radius:0.5rem;background:transparent;color:inherit;font:inherit';
		again.addEventListener('click', () => location.reload());
		notice.append(said, again);
		document.body.append(notice);
	};
	const recover = () => {
		if (sessionStorage.getItem(reloadedKey)) return sayItFailed();
		sessionStorage.setItem(reloadedKey, '1');
		location.reload();
	};
	addEventListener(
		'error',
		(event) => {
			const source = event.target;
			if (!source || source.tagName !== 'SCRIPT' || !source.src.includes('/_app/immutable/')) return;
			recover();
		},
		true
	);
	addEventListener('unhandledrejection', (event) => {
		if (hasStarted()) return;
		const reason = event.reason;
		const said = String((reason && reason.message) || reason).toLowerCase();
		if (phrases.some((phrase) => said.includes(phrase))) recover();
	});
	const started = new MutationObserver(() => {
		if (!hasStarted()) return;
		sessionStorage.removeItem(reloadedKey);
		started.disconnect();
	});
	started.observe(root, { attributes: true, attributeFilter: ['data-hydrated'] });
})();`;

export function withStartFailureRecoveryScript(html: string): string {
	return html.replace(startFailureRecoveryPlaceholder, `<script id="start-failure-recovery">${recovery}</script>`);
}
