export type HostAttachment = { role: 'host'; seenAt: number; isRetired: boolean };
export type ClientAttachment = { role: 'client'; memberID: string };
export type StreamAttachment = { role: 'stream'; streamID: string; isClosing: boolean };
export type SocketAttachment = HostAttachment | ClientAttachment | StreamAttachment;

export const hostTag = 'host';
export const clientTag = 'client';
export const streamTag = 'stream';

export function memberTagOf(memberID: string): string {
	return `member:${memberID}`;
}

export function streamTagOf(streamID: string): string {
	return `stream:${streamID}`;
}

export function socketAttachmentOf(socket: WebSocket): SocketAttachment | null {
	const kept: unknown = socket.deserializeAttachment();
	if (typeof kept !== 'object' || kept === null) return null;
	const record: Record<string, unknown> = { ...kept };
	if (record.role === 'host' && typeof record.seenAt === 'number') {
		return { role: 'host', seenAt: record.seenAt, isRetired: record.isRetired === true };
	}
	if (record.role === 'client' && typeof record.memberID === 'string') {
		return { role: 'client', memberID: record.memberID };
	}
	if (record.role === 'stream' && typeof record.streamID === 'string') {
		return { role: 'stream', streamID: record.streamID, isClosing: record.isClosing === true };
	}
	return null;
}
