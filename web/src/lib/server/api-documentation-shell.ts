import type { ApiDocumentationLanguage } from './openapi';

type ShellCopy = {
	pageTitle: string;
	alternateLanguageName: string;
	modelsSectionLabel: string;
	navigation: { introduction: string; operations: string; endpoints: string };
	search: { label: string; placeholder: string };
	operation: { requestBody: string; responses: string; queryParameters: string };
};

export const shellCopy: Record<ApiDocumentationLanguage, ShellCopy> = {
	ko: {
		pageTitle: '김인턴 API 문서',
		alternateLanguageName: 'English',
		modelsSectionLabel: '스키마',
		navigation: { introduction: '소개', operations: '엔드포인트', endpoints: '엔드포인트' },
		search: { label: '검색', placeholder: '검색어를 입력하세요' },
		operation: { requestBody: '요청 본문', responses: '응답', queryParameters: '쿼리 파라미터' }
	},
	en: {
		pageTitle: 'Intern Kim API Documentation',
		alternateLanguageName: '한국어',
		modelsSectionLabel: 'Schemas',
		navigation: { introduction: 'Introduction', operations: 'Operations', endpoints: 'Endpoints' },
		search: { label: 'Search', placeholder: 'Search' },
		operation: {
			requestBody: 'Request Body',
			responses: 'Responses',
			queryParameters: 'Query Parameters'
		}
	}
};

export function shellConfiguration(language: ApiDocumentationLanguage) {
	const copy = shellCopy[language];
	return {
		pageTitle: copy.pageTitle,
		modelsSectionLabel: copy.modelsSectionLabel,
		favicon: '/icon-192.png',
		documentDownloadType: 'json' as const,
		showToolbar: 'never' as const,
		showDeveloperTools: 'never' as const,
		hideClientButton: true,
		mcp: { disabled: true },
		localization: {
			locale: 'en',
			translations: {
				navigation: copy.navigation,
				search: copy.search,
				operation: copy.operation
			}
		}
	};
}
