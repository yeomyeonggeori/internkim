import type { HTMLLiAttributes } from 'svelte/elements';
import type { HoldToReorderMoves } from '$lib/components/channel/hold-to-reorder';

export type ChannelListSection = 'pinned' | 'channels';

type DraggedChannel = { section: ChannelListSection; channelID: string };

type ChannelDrop = (section: ChannelListSection, draggedID: string, targetID: string) => void;

const sectionAttribute = 'data-reorder-section';
const channelAttribute = 'data-reorder-channel';

function isChannelListSection(value: string | null | undefined): value is ChannelListSection {
	return value === 'pinned' || value === 'channels';
}

export class ChannelListReorder {
	dragged = $state<DraggedChannel | null>(null);
	overChannelID = $state<string | null>(null);

	private readonly drop: ChannelDrop;

	constructor(drop: ChannelDrop) {
		this.drop = drop;
	}

	rowProps(section: ChannelListSection, channelID: string): HTMLLiAttributes & Record<string, unknown> {
		return {
			draggable: 'true',
			[sectionAttribute]: section,
			[channelAttribute]: channelID,
			class: this.overChannelID === channelID ? 'border-primary rounded-md border' : 'rounded-md border border-transparent',
			ondragstart: () => this.start(section, channelID),
			ondragend: () => this.cancel(),
			ondragover: (event: DragEvent) => {
				if (this.enter(section, channelID)) event.preventDefault();
			},
			ondragleave: () => {
				if (this.overChannelID === channelID) this.overChannelID = null;
			},
			ondrop: (event: DragEvent) => {
				event.preventDefault();
				this.finish();
			}
		};
	}

	holdMoves(section: ChannelListSection, channelID: string): HoldToReorderMoves {
		return {
			onStart: () => this.start(section, channelID),
			onMove: (under) => this.enterRowUnder(under),
			onEnd: () => this.finish(),
			onCancel: () => this.cancel()
		};
	}

	private start(section: ChannelListSection, channelID: string): void {
		this.dragged = { section, channelID };
		this.overChannelID = null;
	}

	private enter(section: ChannelListSection, channelID: string): boolean {
		if (!this.dragged || this.dragged.section !== section || this.dragged.channelID === channelID) return false;
		this.overChannelID = channelID;
		return true;
	}

	private enterRowUnder(under: Element | null): void {
		const row = under?.closest(`[${channelAttribute}]`);
		const section = row?.getAttribute(sectionAttribute);
		const channelID = row?.getAttribute(channelAttribute);
		if (isChannelListSection(section) && channelID && this.enter(section, channelID)) return;
		this.overChannelID = null;
	}

	private finish(): void {
		const dragged = this.dragged;
		const targetID = this.overChannelID;
		this.cancel();
		if (dragged && targetID) this.drop(dragged.section, dragged.channelID, targetID);
	}

	private cancel(): void {
		this.dragged = null;
		this.overChannelID = null;
	}
}
