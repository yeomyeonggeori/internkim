import type { CompanyEvent } from '$lib/company-event';

export function createRepeatedArrivalFilter(rememberedLimit: number): (event: CompanyEvent) => boolean {
	const announcedMessageIDs = new Set<string>();
	return (event) => {
		if (event.kind !== 'message.arrived' || !event.messageID) return false;
		if (announcedMessageIDs.has(event.messageID)) return true;
		announcedMessageIDs.add(event.messageID);
		if (announcedMessageIDs.size > rememberedLimit) {
			const oldest = announcedMessageIDs.values().next().value;
			if (oldest !== undefined) announcedMessageIDs.delete(oldest);
		}
		return false;
	};
}
