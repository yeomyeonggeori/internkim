export const previewText = {
	ko: {
		loading: '미리보기를 불러오는 중',
		failed: '미리보기를 불러오지 못했습니다.',
		unsupported: '이 형식은 미리보기를 지원하지 않습니다.',
		unsupportedHint: '원본을 내려받아 확인하세요.',
		tooLarge: '용량이 커서 미리보기를 지원하지 않습니다.',
		pages: '페이지',
		zoomIn: '확대',
		zoomOut: '축소',
		fitWidth: '너비에 맞춤',
		truncated: '처음 1,000행, 60열까지만 표시합니다.',
		emptySheet: '빈 시트입니다.',
		rowNumber: '행',
		slides: '슬라이드',
		htmlPage: 'HTML 미리보기',
		htmlIsolated: '스크립트는 실행하지 않고 표시합니다.'
	},
	en: {
		loading: 'Loading preview',
		failed: 'Could not load the preview.',
		unsupported: 'This format has no preview.',
		unsupportedHint: 'Download the original to open it.',
		tooLarge: 'This file is too large to preview.',
		pages: 'pages',
		zoomIn: 'Zoom in',
		zoomOut: 'Zoom out',
		fitWidth: 'Fit width',
		truncated: 'Showing the first 1,000 rows and 60 columns.',
		emptySheet: 'This sheet is empty.',
		rowNumber: 'Row',
		slides: 'slides',
		htmlPage: 'HTML preview',
		htmlIsolated: 'Shown without running its scripts.'
	}
} as const;
