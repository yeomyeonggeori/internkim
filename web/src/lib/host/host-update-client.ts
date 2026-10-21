import { invokeTool, ToolRefused } from '$lib/public-api-call';
import {
	hostUpdateResultSchema,
	hostVersionGetResultSchema,
	type HostUpdateStarted,
	type HostVersion
} from '$lib/host/host-version';
import type { HostReading } from '$lib/host/host-update-model';

export async function readHostVersion(): Promise<HostVersion> {
	return hostVersionGetResultSchema.parse(await invokeTool('host_version_get', {}));
}

export async function readHostWhileItMayBeAway(): Promise<HostReading> {
	try {
		return { isAnswered: true, version: await readHostVersion() };
	} catch (refusal) {
		if (refusal instanceof ToolRefused) return { isAnswered: false };
		throw refusal;
	}
}

export async function startHostUpdate(targetVersion: string): Promise<HostUpdateStarted> {
	return hostUpdateResultSchema.parse(await invokeTool('host_update', { targetVersion }));
}
