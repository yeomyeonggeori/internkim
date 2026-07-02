export type SiteSection = {
	title: string;
	body: string;
};

export type SiteContent = {
	siteName: string;
	tagline?: string;
	heroActionLabel?: string;
	heroActionHref?: string;
	sections: SiteSection[];
};

export const fallbackContent: SiteContent = {
	siteName: "__SITE_TITLE__",
	tagline: "이 사이트가 무엇에 대한 것인지 한 줄로 소개하세요.",
	heroActionLabel: "자세히 보기",
	heroActionHref: "#section-1",
	sections: [
		{ title: "소개", body: "이 섹션에 요청에 맞는 소개 내용을 채워 주세요." },
		{ title: "연락처", body: "이메일, 전화 등 연락 방법을 적어 주세요." },
	],
};

function isNonEmptyString(value: unknown): value is string {
	return typeof value === "string" && value.length > 0;
}

function isOptionalString(value: unknown): value is string | undefined {
	return value === undefined || typeof value === "string";
}

function isRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === "object" && value !== null;
}

function parseSiteSection(value: unknown): SiteSection | undefined {
	if (!isRecord(value)) return undefined;
	if (!isNonEmptyString(value.title) || !isNonEmptyString(value.body)) return undefined;
	return { title: value.title, body: value.body };
}

function parseSiteContent(value: unknown): SiteContent | undefined {
	if (!isRecord(value)) return undefined;
	if (!isNonEmptyString(value.siteName)) return undefined;
	if (
		!isOptionalString(value.tagline) ||
		!isOptionalString(value.heroActionLabel) ||
		!isOptionalString(value.heroActionHref)
	) {
		return undefined;
	}
	if (!Array.isArray(value.sections)) return undefined;
	const sections = value.sections
		.map(parseSiteSection)
		.filter((section): section is SiteSection => section !== undefined);
	if (sections.length === 0) return undefined;
	return {
		siteName: value.siteName,
		tagline: value.tagline,
		heroActionLabel: value.heroActionLabel,
		heroActionHref: value.heroActionHref,
		sections,
	};
}

export async function loadSiteContent(): Promise<SiteContent> {
	try {
		const response = await fetch("./site-content.json");
		if (!response.ok) return fallbackContent;
		const parsedJSON: unknown = await response.json();
		return parseSiteContent(parsedJSON) ?? fallbackContent;
	} catch {
		return fallbackContent;
	}
}
