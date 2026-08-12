//   bun run host/relay/measure-broadcast-ceiling.ts

export type Shape = 'json' | 'binary';

type Probe = {
	projectURL: string;
	publishableKey: string;
	accessToken: string;
	topic: string;
};

type Verdict = { accepted: boolean; status: number; refusal: string };

const smallestProbe = 1_024;
const largestProbe = 8 * 1024 * 1024;

export function jsonBodyOfExactly(bytes: number): string {
	const around = JSON.stringify({ filler: '' }).length;
	if (bytes <= around) throw new Error(`a json body cannot be ${bytes} bytes`);
	return JSON.stringify({ filler: 'x'.repeat(bytes - around) });
}

export function broadcastURL(projectURL: string, topic: string): string {
	const base = projectURL.replace(/\/+$/, '');
	return `${base}/realtime/v1/api/broadcast/${encodeURIComponent(topic)}/events/probe`;
}

function bodyOf(bytes: number, shape: Shape): string | Uint8Array {
	return shape === 'json' ? jsonBodyOfExactly(bytes) : new Uint8Array(bytes);
}

async function send(probe: Probe, body: string | Uint8Array): Promise<Verdict> {
	const isBinary = typeof body !== 'string';
	const response = await fetch(broadcastURL(probe.projectURL, probe.topic), {
		method: 'POST',
		headers: {
			apikey: probe.publishableKey,
			Authorization: `Bearer ${probe.accessToken}`,
			'Content-Type': isBinary ? 'application/octet-stream' : 'application/json'
		},
		body: isBinary ? (body as unknown as BodyInit) : body
	});
	if (response.status === 202) return { accepted: true, status: 202, refusal: '' };
	return { accepted: false, status: response.status, refusal: (await response.text()).trim().slice(0, 200) };
}

async function largestAccepted(probe: Probe, shape: Shape): Promise<{ bytes: number; refusal: Verdict } | null> {
	const floor = await send(probe, bodyOf(smallestProbe, shape));
	if (!floor.accepted) {
		console.log(`${shape}: even ${smallestProbe} bytes was refused ${floor.status} — ${floor.refusal}`);
		return null;
	}
	const ceiling = await send(probe, bodyOf(largestProbe, shape));
	if (ceiling.accepted) {
		console.log(`${shape}: ${largestProbe} bytes was still accepted; raise largestProbe`);
		return null;
	}

	let accepted = smallestProbe;
	let refused = largestProbe;
	let refusal = ceiling;
	while (refused - accepted > 1) {
		const middle = Math.floor((accepted + refused) / 2);
		const verdict = await send(probe, bodyOf(middle, shape));
		if (verdict.accepted) accepted = middle;
		else {
			refused = middle;
			refusal = verdict;
		}
	}
	return { bytes: accepted, refusal };
}

function required(name: string): string {
	const value = process.env[name]?.trim();
	if (!value) throw new Error(`set ${name}`);
	return value;
}

function asMegabytes(bytes: number): string {
	return `${(bytes / 1000 / 1000).toFixed(3)} MB`;
}

async function report(): Promise<void> {
	const publishableKey = required('SUPABASE_PUBLISHABLE_KEY');
	const probe: Probe = {
		projectURL: required('SUPABASE_URL'),
		publishableKey,
		accessToken: process.env.MEASURE_ACCESS_TOKEN?.trim() || publishableKey,
		topic: process.env.MEASURE_TOPIC?.trim() || 'size-probe'
	};
	console.log(`probing ${broadcastURL(probe.projectURL, probe.topic)}`);
	for (const shape of ['json', 'binary'] as Shape[]) {
		const found = await largestAccepted(probe, shape);
		if (!found) continue;
		console.log(
			`${shape}: largest accepted ${found.bytes} bytes (${asMegabytes(found.bytes)}), ` +
				`refused at ${found.refusal.status} — ${found.refusal.refusal}`
		);
	}
}

if (import.meta.main) await report();
