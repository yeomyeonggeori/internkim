import adapterCloudflare from '@sveltejs/adapter-cloudflare';
import { relative, sep } from 'node:path';

/** @type {import('@sveltejs/kit').Config} */
const config = {
	compilerOptions: {
		runes: ({ filename }) => {
			const relativePath = relative(import.meta.dirname, filename);
			const pathSegments = relativePath.toLowerCase().split(sep);
			const isExternalLibrary = pathSegments.includes('node_modules');
			return isExternalLibrary ? undefined : true;
		}
	},
	kit: {
		adapter: adapterCloudflare(),
		paths: { relative: false },
		...(process.env.INTERNKIM_WEB_REVISION ? { version: { name: process.env.INTERNKIM_WEB_REVISION } } : {})
	}
};

export default config;
