import { getContext, setContext } from 'svelte';

export type ThemeView = 'light' | 'dark' | 'both';
export type ThemeName = 'light' | 'dark';

export type VariantAxis = { name: string; values: string[] };

export type FrameRecord = {
	id: string;
	name: string;
	pageID: string;
	sectionName: string;
	source: string;
	note: string;
	variants: VariantAxis[];
};

export const zoomLevels = [50, 75, 100, 125, 150] as const;

const workspaceKey = Symbol('design-workspace');
const placementKey = Symbol('design-placement');

export class DesignWorkspace {
	themeView = $state<ThemeView>('both');
	zoom = $state<number>(100);
	pageID = $state('');
	selectedID = $state('');
	frames = $state<FrameRecord[]>([]);

	readonly selectedFrame = $derived(this.frames.find((frame) => frame.id === this.selectedID));
	readonly themes: ThemeName[] = $derived(this.themeView === 'both' ? ['light', 'dark'] : [this.themeView]);

	register(frame: FrameRecord): void {
		this.frames = [...this.frames.filter((existing) => existing.id !== frame.id), frame];
	}

	unregister(id: string): void {
		this.frames = this.frames.filter((frame) => frame.id !== id);
	}

	select(id: string): void {
		this.selectedID = id;
		const frame = this.frames.find((candidate) => candidate.id === id);
		if (frame) this.pageID = frame.pageID;
	}

	framesOf(pageID: string): FrameRecord[] {
		return this.frames.filter((frame) => frame.pageID === pageID);
	}
}

export type FramePlacement = { pageID: string; sectionName: string };

export function provideWorkspace(workspace: DesignWorkspace): DesignWorkspace {
	return setContext(workspaceKey, workspace);
}

export function useWorkspace(): DesignWorkspace {
	return getContext<DesignWorkspace>(workspaceKey);
}

export function providePlacement(placement: () => FramePlacement): void {
	setContext(placementKey, placement);
}

export function usePlacement(): () => FramePlacement {
	return getContext<() => FramePlacement>(placementKey);
}

type VariantDefinitions = Record<string, Record<string, unknown>>;

export function variantAxes(definitions: VariantDefinitions): VariantAxis[] {
	return Object.entries(definitions).map(([name, values]) => ({ name, values: Object.keys(values) }));
}

export function variantValues<Values extends Record<string, unknown>>(values: Values): (keyof Values & string)[] {
	return Object.keys(values).filter((key): key is keyof Values & string => key in values);
}
