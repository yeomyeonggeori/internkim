import tailwindcss from '@tailwindcss/vite';
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

const admindTarget = 'http://127.0.0.1:18080';

export default defineConfig({
	plugins: [tailwindcss(), sveltekit()],
	server: {
		proxy: {
			'/.well-known/caldav': admindTarget,
			'/admin/api': admindTarget,
			'/attendance/api': admindTarget,
			'/auth': admindTarget,
			'/calendar/api': admindTarget,
			'/calendar/dav': admindTarget,
			'/calendar/ics': admindTarget,
			'/calendar/oauth': admindTarget,
			'/flow/api': admindTarget,
			'/mail/api': admindTarget,
			'/memory/api': admindTarget,
		}
	}
});
