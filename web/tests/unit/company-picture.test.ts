import { describe, expect, test } from 'bun:test';
import {
	companyPictureFormats,
	companyPictureMegabytes,
	isCompanyPictureFormat,
	largestPictureACompanyCanHave,
	refusalOfCompanyPictureSize
} from '../../src/lib/company/company-picture';

// Where a picture is kept, that the row refuses a colleague who is not an
// administrator, and that what comes back is a signed address rather than a
// path, are the endpoint's now and are covered by the company-picture cases in
// tests/integration/public-api-route.test.ts. What is left here is the pair of
// rules the browser applies before it sends anything, which are pure.
describe('a picture the bucket should not be asked to take', () => {
	test('is refused with its own size, and one that fits is not', () => {
		expect(refusalOfCompanyPictureSize(11 * 1024 * 1024)).toBe('11.0MB');
		expect(refusalOfCompanyPictureSize(1024)).toBe('');
		expect(refusalOfCompanyPictureSize(largestPictureACompanyCanHave)).toBe('');
	});

	test('measures against the megabytes the chooser tells the person about', () => {
		expect(largestPictureACompanyCanHave).toBe(companyPictureMegabytes * 1024 * 1024);
	});
});

describe('the formats a company picture may arrive in', () => {
	test('are the ones the file chooser offers, read past any charset the browser adds', () => {
		for (const format of companyPictureFormats) {
			expect(isCompanyPictureFormat(format)).toBe(true);
			expect(isCompanyPictureFormat(`${format}; charset=binary`)).toBe(true);
		}
	});

	test('do not include a document dressed as one', () => {
		expect(isCompanyPictureFormat('application/pdf')).toBe(false);
		expect(isCompanyPictureFormat('')).toBe(false);
	});
});
