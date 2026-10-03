import XCTest

final class AttendanceChosenTimeTests: XCTestCase {
    private var store: UserDefaults!
    private let suite = "AttendanceChosenTimeTests"

    override func setUp() {
        super.setUp()
        store = UserDefaults(suiteName: suite)
        store.removePersistentDomain(forName: suite)
    }

    override func tearDown() {
        store.removePersistentDomain(forName: suite)
        super.tearDown()
    }

    func testAChosenTimeIsHeldOnlyOnTheDayItWasChosen() {
        AttendanceChosenTime.keep(AttendanceChosenTime(day: "2026-10-03", time: "09:00"), in: store)

        XCTAssertEqual(AttendanceChosenTime.held(today: "2026-10-03", in: store)?.time, "09:00")
        XCTAssertNil(AttendanceChosenTime.held(today: "2026-10-04", in: store))
    }

    func testClearingForgetsTheChosenTime() {
        AttendanceChosenTime.keep(AttendanceChosenTime(day: "2026-10-03", time: "09:00"), in: store)
        AttendanceChosenTime.clear(in: store)

        XCTAssertNil(AttendanceChosenTime.held(today: "2026-10-03", in: store))
    }

    func testTheChosenMomentIsReadInTheCompanyTimeZone() throws {
        let seoul = try XCTUnwrap(TimeZone(identifier: "Asia/Seoul"))
        let moment = try XCTUnwrap(AttendanceChosenTime(day: "2026-10-03", time: "09:00").moment(in: seoul))

        XCTAssertEqual(AttendanceChosenTime.of(moment, in: seoul), AttendanceChosenTime(day: "2026-10-03", time: "09:00"))
        XCTAssertEqual(moment, Date(timeIntervalSince1970: 1_790_985_600))
    }

    func testTheChoosingLinkCarriesTheKindItWasOpenedFor() throws {
        let url = try XCTUnwrap(AttendanceChosenTime.choosingURL(kind: "clock_out"))

        XCTAssertEqual(AttendanceChosenTime.kind(choosingFrom: url), "clock_out")
        XCTAssertNil(AttendanceChosenTime.kind(choosingFrom: try XCTUnwrap(URL(string: "https://example.com/choose-time?kind=clock_in"))))
    }
}
