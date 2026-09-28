const storageKey = 'messenger-collapsed-sections';

const channelSections = ['channels', 'directMessages'] as const;

export type ChannelSection = (typeof channelSections)[number];

function isChannelSection(value: unknown): value is ChannelSection {
	return channelSections.some((section) => section === value);
}

function loadCollapsedSections(): ChannelSection[] {
	if (typeof localStorage === 'undefined') return [];
	try {
		const stored: unknown = JSON.parse(localStorage.getItem(storageKey) ?? '[]');
		return Array.isArray(stored) ? stored.filter(isChannelSection) : [];
	} catch {
		return [];
	}
}

let collapsedSections = $state<ChannelSection[]>(loadCollapsedSections());

export function isSectionOpen(section: ChannelSection): boolean {
	return !collapsedSections.includes(section);
}

export function setSectionOpen(section: ChannelSection, open: boolean): void {
	collapsedSections = open
		? collapsedSections.filter((collapsed) => collapsed !== section)
		: [...collapsedSections, section];
	if (typeof localStorage === 'undefined') return;
	localStorage.setItem(storageKey, JSON.stringify(collapsedSections));
}
