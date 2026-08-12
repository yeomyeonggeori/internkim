import type { ApiDocumentationLanguage } from './openapi';

export function preferredDocumentationLanguage(
	acceptLanguage: string | null
): ApiDocumentationLanguage {
	return acceptLanguage?.toLowerCase().startsWith('en') ? 'en' : 'ko';
}

export function isDocumentationLanguage(
	language: string | undefined
): language is ApiDocumentationLanguage {
	return language === 'ko' || language === 'en';
}
