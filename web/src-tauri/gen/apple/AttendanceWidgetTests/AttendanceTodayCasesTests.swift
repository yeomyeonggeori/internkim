import XCTest

final class AttendanceTodayCasesTests: XCTestCase {
    private struct Cases: Decodable {
        let cases: [Case]
    }

    private struct Case: Decodable {
        let name: String
        let today: String
        let timeZone: String
        let now: String
        let currentTime: String
        let rows: [Row]
        let expected: Expected
    }

    private struct Row: Decodable {
        let kind: String
        let date: String
        let time: String
        let occurredAt: String
        let location: String?
    }

    private struct Expected: Decodable {
        let workedMinutes: Int
        let elapsedMinutes: Int
        let isWorking: Bool
        let location: String?
        let clockInTime: String?
        let clockOutTime: String?
        let bars: [Bar]
    }

    private struct Bar: Decodable, Equatable {
        let location: String?
        let widthPercent: Double
    }

    func testTheWidgetDrawsTheDayTheAttendanceCardDraws() throws {
        let url = try XCTUnwrap(Bundle(for: Self.self).url(forResource: "attendance-today-cases", withExtension: "json"))
        let cases = try JSONDecoder().decode(Cases.self, from: Data(contentsOf: url)).cases
        XCTAssertFalse(cases.isEmpty)

        for day in cases {
            let now = try XCTUnwrap(AttendanceToday.moment(day.now), day.name)
            let zone = try XCTUnwrap(TimeZone(identifier: day.timeZone), day.name)
            XCTAssertEqual(CompanyClock.day(of: now, in: zone), day.today, day.name)
            XCTAssertEqual(CompanyClock.time(of: now, in: zone), day.currentTime, day.name)

            let rows = day.rows.enumerated().map { index, row in
                AttendanceRow(
                    eventID: "event-\(index)",
                    kind: row.kind,
                    date: row.date,
                    time: row.time,
                    occurredAt: row.occurredAt,
                    location: row.location
                )
            }
            let computed = AttendanceToday.of(rows: rows, today: day.today, now: now)

            XCTAssertEqual(computed.workedMinutes, day.expected.workedMinutes, day.name)
            XCTAssertEqual(computed.elapsedMinutes(at: now), day.expected.elapsedMinutes, day.name)
            XCTAssertEqual(computed.isWorking, day.expected.isWorking, day.name)
            XCTAssertEqual(computed.location, day.expected.location, day.name)
            XCTAssertEqual(computed.clockInTime, day.expected.clockInTime, day.name)
            XCTAssertEqual(computed.clockOutTime, day.expected.clockOutTime, day.name)
            XCTAssertEqual(
                computed.bars(currentTime: day.currentTime).map {
                    Bar(location: $0.location, widthPercent: ($0.widthPercent * 10000).rounded() / 10000)
                },
                day.expected.bars,
                day.name
            )
        }
    }
}
