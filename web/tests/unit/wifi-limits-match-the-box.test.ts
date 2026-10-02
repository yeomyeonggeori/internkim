import { expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { maximumNearbyNetworks, maximumSSIDBytes, nearbyNetworkListSchema, wifiOutcomeResultSchema } from '$lib/company/box';

const boxSource = readFileSync('../internal/box/wifi_change.go', 'utf8');

function goConstant(name: string): number {
	const match = boxSource.match(new RegExp(`\\b${name}\\s*=\\s*(\\d+)`));
	if (!match?.[1]) throw new Error(`internal/box/wifi_change.go no longer declares ${name}`);
	return Number(match[1]);
}

test('the web limits on nearby networks equal the ones the box enforces', () => {
	expect(goConstant('maximumNearbyNetworks')).toBe(maximumNearbyNetworks);
	expect(goConstant('maximumSSIDBytes')).toBe(maximumSSIDBytes);
});

test('the web accepts exactly the Wi-Fi outcomes the box reports', () => {
	const boxOutcomes = [...boxSource.matchAll(/\bWifiChange\w+\s+WifiChangeOutcome\s*=\s*"([^"]+)"/g)].map((match) => match[1]);
	expect(boxOutcomes.length).toBeGreaterThan(0);
	expect(boxOutcomes.toSorted()).toEqual([...wifiOutcomeResultSchema.options].toSorted());
});

test('a nearby network list refuses two networks with the same SSID', () => {
	const network = { ssid: 'Office', signalPercent: 50, isSecured: true };
	expect(nearbyNetworkListSchema.safeParse([network]).success).toBe(true);
	expect(nearbyNetworkListSchema.safeParse([network, { ...network, signalPercent: 40 }]).success).toBe(false);
});
