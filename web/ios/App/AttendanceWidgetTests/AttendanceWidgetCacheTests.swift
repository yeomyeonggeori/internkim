import XCTest

final class AttendanceWidgetCacheTests: XCTestCase {
    private let now = Date(timeIntervalSince1970: 1_789_600_000)

    func testOptimisticRowCarriesTheTappedKindAndLocation() throws {
        let seoul = try XCTUnwrap(TimeZone(identifier: "Asia/Seoul"))
        let row = AttendanceWidgetCache.optimisticRow(kind: "clock_in", location: "재택", now: now, timeZone: seoul)

        XCTAssertEqual(row.eventID, "pending")
        XCTAssertEqual(row.kind, "clock_in")
        XCTAssertEqual(row.location, "재택")
        XCTAssertEqual(row.date, CompanyClock.day(of: now, in: seoul))
        XCTAssertEqual(row.time, CompanyClock.time(of: now, in: seoul))
        XCTAssertEqual(AttendanceToday.moment(row.occurredAt), now)
    }

    func testOptimisticRowDropsLocationForClockOut() throws {
        let seoul = try XCTUnwrap(TimeZone(identifier: "Asia/Seoul"))
        let row = AttendanceWidgetCache.optimisticRow(kind: "clock_out", location: "재택", now: now, timeZone: seoul)

        XCTAssertNil(row.location)
    }

    func testEventToRowConvertsUsingTheGivenTimeZoneAcrossMidnightUTC() throws {
        let seoul = try XCTUnwrap(TimeZone(identifier: "Asia/Seoul"))
        let event = AttendanceEvent(
            id: "evt-1",
            kind: "clock_in",
            occurredAt: "2026-09-23T15:30:00Z",
            location: "사무실"
        )

        let row = try XCTUnwrap(AttendanceWidgetCache.row(from: event, timeZone: seoul))

        XCTAssertEqual(row.eventID, "evt-1")
        XCTAssertEqual(row.kind, "clock_in")
        XCTAssertEqual(row.date, "2026-09-24")
        XCTAssertEqual(row.time, "00:30")
        XCTAssertEqual(row.occurredAt, "2026-09-23T15:30:00Z")
        XCTAssertEqual(row.location, "사무실")
    }

    func testEventToRowFailsWhenOccurredAtCannotBeParsed() {
        let event = AttendanceEvent(id: "evt-1", kind: "clock_in", occurredAt: "not-a-date", location: nil)

        XCTAssertNil(AttendanceWidgetCache.row(from: event, timeZone: .current))
    }

    func testRowsShownServeCachedRowsAndThePendingTapWhileTrusted() throws {
        let shown = try XCTUnwrap(
            AttendanceWidgetCache.rowsShown(
                cachedRows: [sampleRow],
                pending: pendingRow,
                rowsTrustedUntil: now.addingTimeInterval(60),
                drawsTap: false,
                now: now
            )
        )

        XCTAssertEqual(shown.map(\.eventID), ["e1", "pending"])
    }

    func testRowsShownSignalARefetchOnceTrustExpiresEvenWithAPendingTap() {
        XCTAssertNil(
            AttendanceWidgetCache.rowsShown(
                cachedRows: [sampleRow],
                pending: pendingRow,
                rowsTrustedUntil: now.addingTimeInterval(-1),
                drawsTap: false,
                now: now
            )
        )
    }

    func testRowsShownSignalARefetchWithNoTrustWindow() {
        XCTAssertNil(AttendanceWidgetCache.rowsShown(cachedRows: [sampleRow], pending: nil, rowsTrustedUntil: nil, drawsTap: false, now: now))
    }

    func testRowsShownServeHeldRowsForATapAfterTrustExpires() throws {
        let shown = try XCTUnwrap(
            AttendanceWidgetCache.rowsShown(
                cachedRows: [sampleRow],
                pending: nil,
                rowsTrustedUntil: now.addingTimeInterval(-1),
                drawsTap: true,
                now: now
            )
        )

        XCTAssertEqual(shown.map(\.eventID), ["e1"])
    }

    func testATapIsDrawnFromCacheOnlyRightAfterIt() {
        XCTAssertTrue(AttendanceWidgetCache.drawsTapFromCache(tappedAt: now.addingTimeInterval(-1), now: now))
        XCTAssertFalse(
            AttendanceWidgetCache.drawsTapFromCache(
                tappedAt: now.addingTimeInterval(-AttendanceWidgetCache.tapDrawnFromCacheFor),
                now: now
            )
        )
        XCTAssertFalse(AttendanceWidgetCache.drawsTapFromCache(tappedAt: nil, now: now))
    }

    func testACacheWrittenBeforeTapsWereRecordedStillDecodes() throws {
        let written = #"{"origin":"https://alpha.example.com","rows":[]}"#

        let decoded = try JSONDecoder().decode(AttendanceWidgetCache.self, from: Data(written.utf8))

        XCTAssertNil(decoded.tappedAt)
        XCTAssertNil(decoded.rowsListedAt)
    }

    func testAppendingReplacesARowTheListAlreadyCarries() {
        let rows = AttendanceWidgetCache.appending(sampleRow, to: [sampleRow])

        XCTAssertEqual(rows.map(\.eventID), ["e1"])
    }

    func testARefusalIsNotUndoneByAReadThatStartedBeforeIt() throws {
        let suite = "AttendanceWidgetCacheTests.\(UUID().uuidString)"
        let store = try XCTUnwrap(UserDefaults(suiteName: suite))
        defer { store.removePersistentDomain(forName: suite) }
        let origin = "https://alpha.example.com"

        AttendanceWidgetCache.amend(origin: origin, in: store) {
            $0.pending = pendingRow
            $0.rowsTrustedUntil = now.addingTimeInterval(AttendanceWidgetCache.rowsTrustedFor)
        }
        let readBeforeRefusal = AttendanceWidgetCache.held(origin: origin, in: store)
        XCTAssertNotNil(readBeforeRefusal.pending)

        AttendanceWidgetCache.amend(origin: origin, in: store) {
            $0.pending = nil
            $0.rowsTrustedUntil = nil
        }
        AttendanceWidgetCache.amend(origin: origin, in: store) {
            $0.settings = CompanySettings(timeZone: "Asia/Seoul", workLocations: [])
            $0.settingsFetchedAt = now
        }

        let settled = AttendanceWidgetCache.held(origin: origin, in: store)
        XCTAssertNil(settled.pending)
        XCTAssertNil(settled.rowsTrustedUntil)
        XCTAssertEqual(settled.settings?.timeZone, "Asia/Seoul")
    }

    func testSettingsAreFreshWithinSixHours() {
        XCTAssertTrue(AttendanceWidgetCache.settingsAreFresh(fetchedAt: now.addingTimeInterval(-6 * 60 * 60 + 1), now: now))
    }

    func testSettingsAreStaleAtSixHours() {
        XCTAssertFalse(AttendanceWidgetCache.settingsAreFresh(fetchedAt: now.addingTimeInterval(-6 * 60 * 60), now: now))
    }

    func testSettingsAreStaleWhenNeverFetched() {
        XCTAssertFalse(AttendanceWidgetCache.settingsAreFresh(fetchedAt: nil, now: now))
    }

    func testAnOriginMismatchIsTreatedAsEmpty() {
        let held = AttendanceWidgetCache(
            origin: "https://alpha.example.com",
            settings: nil,
            settingsFetchedAt: now,
            rows: [sampleRow],
            rowsTrustedUntil: now,
            pending: sampleRow
        )

        let resolved = AttendanceWidgetCache.resolve(decoded: held, origin: "https://beta.example.com")

        XCTAssertEqual(resolved.origin, "https://beta.example.com")
        XCTAssertTrue(resolved.rows.isEmpty)
        XCTAssertNil(resolved.settings)
        XCTAssertNil(resolved.rowsTrustedUntil)
        XCTAssertNil(resolved.pending)
    }

    func testAMatchingOriginIsKept() {
        let held = AttendanceWidgetCache.empty(origin: "https://alpha.example.com")

        let resolved = AttendanceWidgetCache.resolve(decoded: held, origin: "https://alpha.example.com")

        XCTAssertEqual(resolved.origin, held.origin)
    }

    func testNoDecodedCacheIsTreatedAsEmpty() {
        let resolved = AttendanceWidgetCache.resolve(decoded: nil, origin: "https://alpha.example.com")

        XCTAssertTrue(resolved.rows.isEmpty)
        XCTAssertEqual(resolved.origin, "https://alpha.example.com")
    }

    private var sampleRow: AttendanceRow {
        AttendanceRow(eventID: "e1", kind: "clock_in", date: "2026-09-23", time: "09:00", occurredAt: "2026-09-23T00:00:00Z", location: nil)
    }

    private var pendingRow: AttendanceRow {
        AttendanceRow(eventID: "pending", kind: "clock_out", date: "2026-09-23", time: "18:00", occurredAt: "2026-09-23T09:00:00Z", location: nil)
    }
}
