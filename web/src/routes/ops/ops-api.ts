import type { Job, JobEvent, NewTarget, OpsTarget, TargetStatus } from './ops-types';

async function readJSON<T>(response: Response): Promise<T> {
	if (!response.ok) {
		const message = await readErrorMessage(response);
		throw new Error(message);
	}
	return (await response.json()) as T;
}

async function readErrorMessage(response: Response): Promise<string> {
	try {
		const document = (await response.json()) as { error?: string };
		return document.error ?? `${response.status} ${response.statusText}`;
	} catch {
		return `${response.status} ${response.statusText}`;
	}
}

export async function fetchTargets(): Promise<OpsTarget[]> {
	const response = await fetch('/api/targets');
	return readJSON<OpsTarget[]>(response);
}

export async function createTarget(target: NewTarget): Promise<OpsTarget> {
	const response = await fetch('/api/targets', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(target)
	});
	return readJSON<OpsTarget>(response);
}

export async function fetchTargetStatus(targetID: string): Promise<TargetStatus> {
	const response = await fetch(`/api/targets/${encodeURIComponent(targetID)}/status`);
	return readJSON<TargetStatus>(response);
}

export async function startJob(targetID: string, action: string): Promise<Job> {
	const response = await fetch(`/api/targets/${encodeURIComponent(targetID)}/jobs`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ action })
	});
	return readJSON<Job>(response);
}

export function streamJobEvents(jobID: string, onEvent: (event: JobEvent) => void, onError: (message: string) => void): EventSource {
	const eventSource = new EventSource(`/api/jobs/${encodeURIComponent(jobID)}/events`);
	eventSource.addEventListener('message', (event) => {
		onEvent(JSON.parse((event as MessageEvent).data) as JobEvent);
	});
	eventSource.onerror = () => {
		onError('job event stream disconnected');
	};
	return eventSource;
}
