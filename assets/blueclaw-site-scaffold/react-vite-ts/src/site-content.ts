export type BlockVariant = "hero" | "features" | "prose" | "cta" | "faq" | "contact";

export type BlockItem = {
	title: string;
	body: string;
};

export type Block = {
	variant: BlockVariant;
	title?: string;
	body?: string;
	items?: BlockItem[];
	actionLabel?: string;
	actionHref?: string;
};

export type SiteContent = {
	siteName: string;
	tagline?: string;
	blocks: Block[];
};

export const fallbackContent: SiteContent = {
	siteName: "__SITE_TITLE__",
	tagline: "이 사이트가 무엇에 대한 것인지 한 줄로 소개하세요.",
	blocks: [
		{
			variant: "hero",
			title: "__SITE_TITLE__",
			body: "이 사이트가 무엇에 대한 것인지 한 줄로 소개하세요.",
			actionLabel: "자세히 보기",
			actionHref: "#block-2",
		},
		{
			variant: "prose",
			title: "소개",
			body: "이 섹션에 요청에 맞는 소개 내용을 채워 주세요.",
		},
		{
			variant: "contact",
			title: "연락처",
			body: "이메일, 전화 등 연락 방법을 적어 주세요.",
		},
	],
};

const knownBlockVariants: readonly string[] = ["hero", "features", "prose", "cta", "faq", "contact"];

function isBlockVariant(value: unknown): value is BlockVariant {
	return typeof value === "string" && knownBlockVariants.includes(value);
}

function isNonEmptyString(value: unknown): value is string {
	return typeof value === "string" && value.length > 0;
}

function isOptionalString(value: unknown): value is string | undefined {
	return value === undefined || typeof value === "string";
}

function isRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === "object" && value !== null;
}

type LegacySiteSection = {
	title: string;
	body: string;
};

function parseLegacySiteSection(value: unknown): LegacySiteSection | undefined {
	if (!isRecord(value)) return undefined;
	if (!isNonEmptyString(value.title) || !isNonEmptyString(value.body)) return undefined;
	return { title: value.title, body: value.body };
}

function parseBlockItem(value: unknown): BlockItem | undefined {
	if (!isRecord(value)) return undefined;
	if (!isNonEmptyString(value.title) || !isNonEmptyString(value.body)) return undefined;
	return { title: value.title, body: value.body };
}

function parseBlockItems(value: unknown): BlockItem[] | undefined {
	if (!Array.isArray(value)) return undefined;
	const items = value.map(parseBlockItem).filter((item): item is BlockItem => item !== undefined);
	return items.length > 0 ? items : undefined;
}

function parseBlock(value: unknown): Block | undefined {
	if (!isRecord(value)) return undefined;
	if (!isBlockVariant(value.variant)) return undefined;
	if (!isOptionalString(value.title) || !isOptionalString(value.body)) return undefined;
	if (!isOptionalString(value.actionLabel) || !isOptionalString(value.actionHref)) return undefined;
	return {
		variant: value.variant,
		title: value.title,
		body: value.body,
		items: parseBlockItems(value.items),
		actionLabel: value.actionLabel,
		actionHref: value.actionHref,
	};
}

function parseBlocks(value: unknown[]): Block[] {
	return value.map(parseBlock).filter((block): block is Block => block !== undefined);
}

function normalizeLegacySections(value: Record<string, unknown>): Block[] {
	const siteName = value.siteName;
	const tagline = value.tagline;
	const heroActionLabel = value.heroActionLabel;
	const heroActionHref = value.heroActionHref;
	const legacySections = Array.isArray(value.sections)
		? value.sections.map(parseLegacySiteSection).filter((section): section is LegacySiteSection => section !== undefined)
		: [];
	const heroBlock: Block = {
		variant: "hero",
		title: isNonEmptyString(siteName) ? siteName : undefined,
		body: isOptionalString(tagline) ? tagline : undefined,
		actionLabel: isOptionalString(heroActionLabel) ? heroActionLabel : undefined,
		actionHref: isOptionalString(heroActionHref) ? heroActionHref : undefined,
	};
	const proseBlocks: Block[] = legacySections.map((section) => ({
		variant: "prose",
		title: section.title,
		body: section.body,
	}));
	return [heroBlock, ...proseBlocks];
}

function parseSiteContent(value: unknown): SiteContent | undefined {
	if (!isRecord(value)) return undefined;
	if (!isNonEmptyString(value.siteName)) return undefined;
	if (!isOptionalString(value.tagline)) return undefined;
	const blocks = Array.isArray(value.blocks) ? parseBlocks(value.blocks) : normalizeLegacySections(value);
	if (blocks.length === 0) return undefined;
	return {
		siteName: value.siteName,
		tagline: value.tagline,
		blocks,
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
