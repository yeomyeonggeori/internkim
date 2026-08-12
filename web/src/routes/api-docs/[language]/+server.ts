import { error } from '@sveltejs/kit';
import { ScalarApiReference } from '@scalar/sveltekit';
import type { RequestHandler } from './$types';
import { isDocumentationLanguage } from '$lib/server/api-documentation-language';
import { apiDocumentationTheme } from '$lib/server/api-documentation-theme';
import { shellConfiguration, shellCopy } from '$lib/server/api-documentation-shell';
import type { ApiDocumentationLanguage } from '$lib/server/openapi';

const scalarBundle = 'https://cdn.jsdelivr.net/npm/@scalar/api-reference@1.64.1';

const switcherStyle =
	'<style>.api-language-switcher{position:fixed;right:1rem;bottom:1rem;z-index:1000;' +
	'border:1px solid var(--scalar-border-color);border-radius:var(--scalar-radius);' +
	'padding:.35rem .7rem;background:var(--scalar-background-1);color:var(--scalar-color-2);' +
	'font:500 13px var(--scalar-font);text-decoration:none}' +
	'.api-language-switcher:hover{color:var(--scalar-color-1)}</style>';

function languageSwitcher(language: ApiDocumentationLanguage): string {
	const alternate = language === 'ko' ? 'en' : 'ko';
	return `<a class="api-language-switcher" href="/api-docs/${alternate}" hreflang="${alternate}">${shellCopy[language].alternateLanguageName}</a>`;
}

function alternateLinks(): string {
	return [
		'<link rel="alternate" hreflang="ko" href="/api-docs/ko" />',
		'<link rel="alternate" hreflang="en" href="/api-docs/en" />',
		'<link rel="alternate" hreflang="x-default" href="/api-docs/ko" />'
	].join('');
}

function localize(apiReferenceHTML: string, language: ApiDocumentationLanguage): string {
	return apiReferenceHTML
		.replace('<html>', `<html lang="${language}">`)
		.replace('</head>', `${alternateLinks()}${switcherStyle}</head>`)
		.replace('<body>', `<body>${languageSwitcher(language)}`);
}

export const GET: RequestHandler = async ({ params }) => {
	if (!isDocumentationLanguage(params.language)) error(404, 'API documentation not found');
	const renderApiReference = ScalarApiReference({
		url: `/openapi/${params.language}.json`,
		cdn: scalarBundle,
		customCss: apiDocumentationTheme,
		...shellConfiguration(params.language)
	});
	const apiReferenceHTML = await renderApiReference().text();
	return new Response(localize(apiReferenceHTML, params.language), {
		headers: {
			'Content-Type': 'text/html; charset=utf-8',
			'Cache-Control': 'public, max-age=0, s-maxage=3600'
		}
	});
};
