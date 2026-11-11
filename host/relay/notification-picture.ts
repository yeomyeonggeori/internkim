import { reasonOf } from './failure';
import { readTimeLimitMilliseconds, withinTimeLimit } from './time-limit';

export const largestNotificationPictureBytes = 5 * 1024 * 1024;

const pictureContentTypes = new Set(['image/jpeg', 'image/png', 'image/gif']);
const pictureDigestShape = /^[0-9a-f]{64}$/;

export type NotificationPictureSources = {
	pathOf: (digest: string) => string;
	keptSizeOf: (path: string) => Promise<number | null>;
	report: (line: string) => void;
	timeLimitMilliseconds?: number;
};

export type ArrivedPicture = { pictureDigest: string; pictureContentType: string };

export async function notificationPicturePathOf(
	arrived: ArrivedPicture,
	sources: NotificationPictureSources
): Promise<string> {
	if (!pictureContentTypes.has(arrived.pictureContentType)) return '';
	if (!pictureDigestShape.test(arrived.pictureDigest)) return '';

	const path = sources.pathOf(arrived.pictureDigest);
	try {
		const sizeBytes = await withinTimeLimit(
			sources.keptSizeOf(path),
			sources.timeLimitMilliseconds ?? readTimeLimitMilliseconds
		);
		return isShowable(sizeBytes) ? path : '';
	} catch (thrown) {
		sources.report(`message picture ${arrived.pictureDigest} not shown in the notification: ${reasonOf(thrown)}`);
		return '';
	}
}

function isShowable(sizeBytes: number | null): boolean {
	if (sizeBytes === null) return false;
	return sizeBytes <= largestNotificationPictureBytes;
}
