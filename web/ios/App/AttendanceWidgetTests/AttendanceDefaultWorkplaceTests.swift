import XCTest

final class AttendanceDefaultWorkplaceTests: XCTestCase {
    private let office = WorkLocation(name: "Office", color: "#112233")
    private let home = WorkLocation(name: "Home", color: nil)

    func testNothingConfiguredKeepsTakingTheFirstWorkplace() {
        XCTAssertEqual(AttendanceDefaultWorkplace.of(configured: nil, among: [office, home]), .chosen(office))
    }

    func testAnEmptyConfiguredNameIsTreatedAsNothingConfigured() {
        XCTAssertEqual(AttendanceDefaultWorkplace.of(configured: "", among: [office, home]), .chosen(office))
    }

    func testACompanyWithNoWorkplaceChoosesNothing() {
        XCTAssertEqual(AttendanceDefaultWorkplace.of(configured: nil, among: []), AttendanceDefaultWorkplace.none)
    }

    func testTheConfiguredWorkplaceWinsOverTheFirstOne() {
        XCTAssertEqual(AttendanceDefaultWorkplace.of(configured: "Home", among: [office, home]), .chosen(home))
    }

    func testAWorkplaceTheCompanyNoLongerHasAsksForAChoiceInsteadOfClockingIn() {
        let choice = AttendanceDefaultWorkplace.of(configured: "Warehouse", among: [office, home])
        XCTAssertEqual(choice, .gone("Warehouse"))
        XCTAssertNil(choice.location)
        XCTAssertTrue(choice.needsChoice)
    }

    func testTheOtherWorkplacesLeaveOutWhicheverIsDefault() {
        XCTAssertEqual(AttendanceDefaultWorkplace.others(besides: home, among: [office, home]), [office])
    }

    func testTheOtherWorkplacesKeepEveryOneWhenNoneIsDefault() {
        XCTAssertEqual(AttendanceDefaultWorkplace.others(besides: nil, among: [office, home]), [office, home])
    }
}

final class AttendanceWorkplaceQueryTests: XCTestCase {
    func testAStoredWorkplaceResolvesEvenWhenTheCompanyNoLongerRegistersIt() async throws {
        let resolved = try await AttendanceWorkplaceQuery().entities(for: ["Warehouse"])
        XCTAssertEqual(resolved.map(\.name), ["Warehouse"])
    }
}
