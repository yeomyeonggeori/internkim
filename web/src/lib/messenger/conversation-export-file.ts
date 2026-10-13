export function printDocument(html: string): void {
	const frame = document.createElement('iframe');
	frame.setAttribute('aria-hidden', 'true');
	frame.style.position = 'fixed';
	frame.style.width = '0';
	frame.style.height = '0';
	frame.style.border = '0';
	frame.addEventListener(
		'load',
		() => {
			const view = frame.contentWindow;
			if (!view) throw new Error('the print frame has no window to print from');
			view.addEventListener('afterprint', () => frame.remove(), { once: true });
			view.focus();
			view.print();
		},
		{ once: true }
	);
	frame.srcdoc = html;
	document.body.appendChild(frame);
}
