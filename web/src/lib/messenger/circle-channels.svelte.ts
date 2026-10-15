import { circleName } from '$lib/data-room/model';
import { circleListResultSchema, type ListedCircle } from '$lib/data-room/schemas';
import { currentLocale } from '$lib/i18n/locale.svelte';
import { invokeTool } from '$lib/public-api-call';
import { isSupabaseConfigured } from '$lib/supabase-session';

class CircleChannels {
	#circleOfChannel = $state<ReadonlyMap<string, ListedCircle>>(new Map());

	circleNameOf(channelID: string): string | undefined {
		const circle = this.#circleOfChannel.get(channelID);
		return circle && circleName(circle, currentLocale.value);
	}

	async load(): Promise<void> {
		if (!isSupabaseConfigured()) return;
		const { circles } = circleListResultSchema.parse(await invokeTool('circle_list', {}));
		this.#circleOfChannel = new Map(
			circles.flatMap((circle) => (circle.channelID ? [[circle.channelID, circle] as const] : []))
		);
	}
}

export const circleChannels = new CircleChannels();
