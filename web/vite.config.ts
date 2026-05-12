import tailwindcss from '@tailwindcss/vite';
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [tailwindcss(), sveltekit()],
	server: {
		proxy: {
			'/admin': 'http://127.0.0.1:18080',
			'/calendar/api': 'http://127.0.0.1:18080',
			'/calendar/ics': 'http://127.0.0.1:18080',
			'/calendar/dav': 'http://127.0.0.1:18080',
			'/flow': 'http://127.0.0.1:18080',
			'/mail': 'http://127.0.0.1:18080',
			'/.well-known': 'http://127.0.0.1:18080'
		}
	}
});
