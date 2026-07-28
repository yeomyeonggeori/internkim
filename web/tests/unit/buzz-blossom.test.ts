import { describe, expect, test } from "bun:test";
import { blossomBaseURL, imetaTag } from "../../src/lib/buzz-blossom";

describe("buzz blossom helpers", () => {
	test("derives the http(s) blossom base from the relay ws url", () => {
		expect(blossomBaseURL("wss://relay.example.test")).toBe("https://relay.example.test");
		expect(blossomBaseURL("ws://127.0.0.1:3000")).toBe("http://127.0.0.1:3000");
		expect(blossomBaseURL("https://relay.example.test")).toBe("https://relay.example.test");
	});

	test("builds a NIP-92 imeta tag from a blob descriptor", () => {
		const tag = imetaTag({ url: "http://r/abc.png", sha256: "deadbeef", size: 1234, mimeType: "image/png" });
		expect(tag).toEqual(["imeta", "url http://r/abc.png", "m image/png", "x deadbeef", "size 1234"]);
	});
});
