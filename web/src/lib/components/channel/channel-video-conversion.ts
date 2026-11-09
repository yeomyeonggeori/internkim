import { ALL_FORMATS, BlobSource, BufferTarget, Conversion, Input, Mp4OutputFormat, Output } from 'mediabunny';
import type { ChannelOutgoingAttachment } from './channel-api';

const relayVideoType = 'video/mp4';

export async function convertVideoForRelay(file: File): Promise<ChannelOutgoingAttachment> {
	const input = new Input({ formats: ALL_FORMATS, source: new BlobSource(file) });
	const output = new Output({
		format: new Mp4OutputFormat({ fastStart: 'in-memory' }),
		target: new BufferTarget()
	});
	try {
		const conversion = await Conversion.init({
			input,
			output,
			video: { codec: 'avc' },
			audio: { codec: 'aac' },
			showWarnings: false
		});
		if (!conversion.isValid || conversion.discardedTracks.length > 0) {
			const reasons = conversion.discardedTracks.map((track) => `${track.track.type} track: ${track.reason}`);
			throw new Error(
				`video ${file.name} cannot be converted to H.264/AAC MP4 in this browser (${reasons.join(', ') || 'no convertible track'}); try another browser or export it as H.264 MP4`
			);
		}
		await conversion.execute();
		const buffer = output.target.buffer;
		if (!buffer) throw new Error(`video ${file.name} conversion finished without output`);
		return {
			filename: `${file.name.replace(/\.[^.]+$/, '') || 'video'}.mp4`,
			contentType: relayVideoType,
			content: new Blob([buffer], { type: relayVideoType })
		};
	} finally {
		input.dispose();
	}
}
