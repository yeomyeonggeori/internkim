import adapterCloudflare from '@sveltejs/adapter-cloudflare';
import adapterStatic from '@sveltejs/adapter-static';
import { relative, sep } from 'node:path';

const isBoard = process.env.BUILD_TARGET === 'board';

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
		adapter: isBoard
			? adapterStatic({ pages: '../build/board-ui', assets: '../build/board-ui', fallback: 'index.html' })
			: adapterCloudflare(),
		...(isBoard && { paths: { relative: true } })
	}
};

export default config;
