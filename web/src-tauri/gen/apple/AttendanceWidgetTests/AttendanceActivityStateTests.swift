import XCTest

@available(iOS 16.1, *)
final class AttendanceActivityStateTests: XCTestCase {
    private let clockedInAt = Date(timeIntervalSince1970: 1_789_621_320)

    func testTheTimerStartsAtWhatTheDayAlreadyHolds() {
        let state = AttendanceActivityAttributes.ContentState(
            startedAt: clockedInAt.timeIntervalSince1970,
            earlierMinutes: 300,
            location: "사무실"
        )
        XCTAssertEqual(state.countingFrom, clockedInAt.addingTimeInterval(-5 * 60 * 60))
        XCTAssertEqual(state.startedMoment, clockedInAt)
    }

    func testTheFirstShiftOfADayStartsAtTheClockIn() {
        let state = AttendanceActivityAttributes.ContentState(
            startedAt: clockedInAt.timeIntervalSince1970,
            earlierMinutes: 0,
            location: ""
        )
        XCTAssertEqual(state.countingFrom, clockedInAt)
    }

    func testACardLeftOverFromBeforeThisShippedStillDecodes() throws {
        let written = Data(#"{"startedAt":1789621320,"location":"재택"}"#.utf8)
        let state = try JSONDecoder().decode(AttendanceActivityAttributes.ContentState.self, from: written)
        XCTAssertEqual(state.countingFrom, clockedInAt)
        XCTAssertEqual(state.location, "재택")
    }
}
