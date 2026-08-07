import { defineConfig } from 'svelte-docsmith';

export const siteConfig = defineConfig({
	title: 'InternKim',
	description:
		'A coworker your company runs itself, working from the messenger you already use, on a computer you already leave on.',
	url: 'https://docs.intern.kim',
	nav: [{ label: 'Docs', href: '/docs/introduction' }],
	footer: {
		copyright: `© ${new Date().getFullYear()} InternKim`
	}
});
