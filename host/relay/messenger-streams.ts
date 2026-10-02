import {
	binaryStreamFrame,
	bytesOfBinaryFrame,
	isBinaryFrameTooBig,
	isFrameTooBig,
	closeReasonThatFits,
	streamClose,
	streamCloseCode,
	streamFrame,
	type StreamMessage,
	type StreamOpen
} from '../../workers/connection-gateway/src/host-protocol';
import type { FairOutbox } from './fair-outbox';

export type RelaySocketOpener = (url: string, headers: Record<string, string>) => WebSocket;

type SocketTakingHeaders = new (url: string, options: { headers: Record<string, string> }) => WebSocket;

export const openRelaySocket: RelaySocketOpener = (url, headers) => {
	const HeaderWebSocket = WebSocket as unknown as SocketTakingHeaders;
	return new HeaderWebSocket(url, { headers });
};

type Stream = { socket: WebSocket; waiting: (string | Uint8Array<ArrayBuffer>)[]; isOpen: boolean };

export class MessengerStreams {
	private readonly streams = new Map<string, Stream>();

	constructor(
		private readonly relayURL: string,
		private readonly outbox: FairOutbox,
		private readonly report: (line: string) => void,
		private readonly openSocket: RelaySocketOpener = openRelaySocket
	) {}

	get openCount(): number {
		return this.streams.size;
	}

	take(message: StreamMessage): void {
		if (message.kind === 'stream.open') return this.open(message);
		const stream = this.streams.get(message.streamID);
		if (!stream) {
			if (message.kind === 'stream.frame') this.tellTheGateway(message.streamID, streamCloseCode.normal, 'the relay connection has gone');
			return;
		}
		if (message.kind === 'stream.close') return this.closeRelaySide(message.streamID, stream, message.code, message.reason);
		this.sendToRelay(stream, message.isBinary ? bytesOfBinaryFrame(message) : message.data);
	}

	closeEverything(): void {
		for (const [streamID, stream] of this.streams) {
			this.closeRelaySide(streamID, stream, streamCloseCode.hostGone, 'the gateway connection went away');
		}
		this.outbox.forgetEverything();
	}

	private open(open: StreamOpen): void {
		const headers: Record<string, string> = { Host: open.host };
		if (open.forwardedFor) headers['X-Forwarded-For'] = open.forwardedFor;
		const socket = this.openSocket(`${this.relayURL.replace(/\/+$/, '')}${open.path}`, headers);
		socket.binaryType = 'arraybuffer';
		const stream: Stream = { socket, waiting: [], isOpen: false };
		this.streams.set(open.streamID, stream);
		socket.addEventListener('open', () => this.onRelayOpen(stream));
		socket.addEventListener('message', (message) => this.onRelayFrame(open.streamID, stream, message.data));
		socket.addEventListener('close', (closed) => this.onRelayClose(open.streamID, stream, closed.code, closed.reason));
		socket.addEventListener('error', () => this.report(`the messenger relay refused stream ${open.streamID} for ${open.host}`));
	}

	private onRelayOpen(stream: Stream): void {
		stream.isOpen = true;
		for (const frame of stream.waiting.splice(0)) stream.socket.send(frame);
	}

	private sendToRelay(stream: Stream, frame: string | Uint8Array<ArrayBuffer>): void {
		if (!stream.isOpen) {
			stream.waiting.push(frame);
			return;
		}
		stream.socket.send(frame);
	}

	private onRelayFrame(streamID: string, stream: Stream, data: unknown): void {
		if (this.streams.get(streamID) !== stream) return;
		const document = this.frameDocumentOf(streamID, data);
		if (document === 'too big') {
			this.closeBothSides(streamID, stream, streamCloseCode.frameTooBig, 'a frame may carry at most 512 KiB');
			return;
		}
		if (document && !this.outbox.push(streamID, document)) {
			this.closeBothSides(streamID, stream, streamCloseCode.tryAgainLater, 'the app is reading slower than the relay is sending');
		}
	}

	private frameDocumentOf(streamID: string, data: unknown): string | 'too big' | null {
		if (typeof data === 'string') return isFrameTooBig(data) ? 'too big' : streamFrame(streamID, data);
		if (data instanceof ArrayBuffer) {
			return isBinaryFrameTooBig(data) ? 'too big' : binaryStreamFrame(streamID, new Uint8Array(data));
		}
		return null;
	}

	private onRelayClose(streamID: string, stream: Stream, code: number, reason: string): void {
		if (this.streams.get(streamID) !== stream) return;
		this.streams.delete(streamID);
		this.tellTheGateway(streamID, code, reason);
	}

	private closeBothSides(streamID: string, stream: Stream, code: number, reason: string): void {
		this.closeRelaySide(streamID, stream, code, reason);
		this.outbox.forget(streamID);
		this.tellTheGateway(streamID, code, reason);
	}

	private closeRelaySide(streamID: string, stream: Stream, code: number, reason: string): void {
		this.streams.delete(streamID);
		const clientCode = code >= 3000 && code <= 4999 ? code : streamCloseCode.normal;
		stream.socket.close(clientCode, closeReasonThatFits(reason));
	}

	private tellTheGateway(streamID: string, code: number, reason: string): void {
		this.outbox.push(streamID, streamClose(streamID, code, reason));
	}
}
