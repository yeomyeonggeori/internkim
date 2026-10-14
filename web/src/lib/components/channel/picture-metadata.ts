const jpegMarkerPrefix = 0xff;
const jpegStartOfScan = 0xda;
const jfifApplicationMarker = 0xe0;
const adobeApplicationMarker = 0xee;
const commentMarker = 0xfe;
const canonicalAdobePayloadLength = 12;
const jfifFixedPayloadLength = 14;

const pngSignatureLength = 8;
const pngChunkOverheadLength = 12;
const pngMetadataChunkTypes = new Set(['iCCP', 'eXIf', 'tEXt', 'zTXt', 'iTXt']);

const textEncoder = new TextEncoder();

export function withoutPictureMetadata(bytes: Uint8Array<ArrayBuffer>, contentType: string): Uint8Array<ArrayBuffer> {
	if (contentType === 'image/jpeg') return withoutJpegMetadata(bytes);
	if (contentType === 'image/png') return withoutPngMetadata(bytes);
	return bytes;
}

function withoutJpegMetadata(bytes: Uint8Array<ArrayBuffer>): Uint8Array<ArrayBuffer> {
	if (bytes[0] !== jpegMarkerPrefix || bytes[1] !== 0xd8) return bytes;
	const kept: Uint8Array<ArrayBuffer>[] = [bytes.subarray(0, 2)];
	let offset = 2;
	while (offset + 4 <= bytes.length) {
		if (bytes[offset] !== jpegMarkerPrefix) return bytes;
		const marker = bytes[offset + 1];
		if (marker === jpegStartOfScan) {
			kept.push(bytes.subarray(offset));
			return joined(kept);
		}
		const segmentEnd = offset + 2 + ((bytes[offset + 2] << 8) | bytes[offset + 3]);
		if (segmentEnd > bytes.length) return bytes;
		const segment = bytes.subarray(offset, segmentEnd);
		if (!isJpegMetadataSegment(marker, segment.subarray(4))) kept.push(segment);
		offset = segmentEnd;
	}
	return bytes;
}

function isJpegMetadataSegment(marker: number, payload: Uint8Array): boolean {
	if (marker === jfifApplicationMarker) return !isCanonicalJfif(payload);
	if (marker === adobeApplicationMarker) return !isCanonicalAdobe(payload);
	return (marker >= 0xe1 && marker <= 0xef) || marker === commentMarker;
}

function isCanonicalJfif(payload: Uint8Array): boolean {
	if (payload.length < jfifFixedPayloadLength) return false;
	if (!startsWith(payload, 'JFIF\0')) return false;
	return payload.length === jfifFixedPayloadLength + 3 * payload[12] * payload[13];
}

function isCanonicalAdobe(payload: Uint8Array): boolean {
	return payload.length === canonicalAdobePayloadLength && startsWith(payload, 'Adobe');
}

function withoutPngMetadata(bytes: Uint8Array<ArrayBuffer>): Uint8Array<ArrayBuffer> {
	if (bytes.length < pngSignatureLength) return bytes;
	const kept: Uint8Array<ArrayBuffer>[] = [bytes.subarray(0, pngSignatureLength)];
	const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
	let offset = pngSignatureLength;
	while (offset < bytes.length) {
		if (offset + pngChunkOverheadLength > bytes.length) return bytes;
		const chunkEnd = offset + pngChunkOverheadLength + view.getUint32(offset);
		if (chunkEnd > bytes.length) return bytes;
		const chunkType = String.fromCharCode(...bytes.subarray(offset + 4, offset + 8));
		if (!pngMetadataChunkTypes.has(chunkType)) kept.push(bytes.subarray(offset, chunkEnd));
		offset = chunkEnd;
	}
	return joined(kept);
}

function startsWith(bytes: Uint8Array, prefix: string): boolean {
	const expected = textEncoder.encode(prefix);
	return expected.every((byte, index) => bytes[index] === byte);
}

function joined(parts: Uint8Array<ArrayBuffer>[]): Uint8Array<ArrayBuffer> {
	const total = parts.reduce((length, part) => length + part.length, 0);
	const result = new Uint8Array(total);
	let offset = 0;
	for (const part of parts) {
		result.set(part, offset);
		offset += part.length;
	}
	return result;
}
