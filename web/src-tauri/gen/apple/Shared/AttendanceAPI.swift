import Foundation

struct AttendanceRow: Codable {
    let eventID: String
    let kind: String
    let date: String
    let time: String
    let occurredAt: String
    let location: String?
}

struct AttendanceList: Decodable {
    let serverTime: String
    let attendance: [AttendanceRow]
}

struct AttendanceEvent: Decodable {
    let id: String
    let kind: String
    let occurredAt: String
    let location: String?
}

struct AttendanceWrite: Decodable {
    let status: String
    let eventID: String?
    let event: AttendanceEvent?
}

struct WorkLocation: Codable, Equatable {
    let name: String
    let color: String?
}

struct CompanySettings: Codable {
    let timeZone: String
    let workLocations: [WorkLocation]

    var companyTimeZone: TimeZone {
        TimeZone(identifier: timeZone) ?? .current
    }
}

enum AttendanceAPIFailure: LocalizedError {
    case noKey
    case refused(String)

    var errorDescription: String? {
        switch self {
        case .noKey: return String(localized: "Sign in to the app to turn the widget on")
        case let .refused(said): return said
        }
    }
}

struct AttendanceAPI {
    let credential: AttendanceWidgetCredential

    static func held() throws -> AttendanceAPI {
        guard let credential = AttendanceWidgetStore.credential() else { throw AttendanceAPIFailure.noKey }
        return AttendanceAPI(credential: credential)
    }

    func since(yesterdayOf now: Date, in timeZone: TimeZone) async throws -> AttendanceList {
        let yesterday = now.addingTimeInterval(-24 * 60 * 60)
        return try await invoke("attendance_list", input: [
            "from": CompanyClock.day(of: yesterday, in: timeZone),
            "to": CompanyClock.day(of: now, in: timeZone)
        ])
    }

    func settings() async throws -> CompanySettings {
        try await invoke("company_settings_get", input: [:])
    }

    func clock(kind: String, location: String?) async throws -> AttendanceWrite {
        var input: [String: Any] = ["kind": kind]
        if let location, !location.isEmpty, kind == "clock_in" { input["location"] = location }
        return try await invoke("attendance_add", input: input)
    }

    private func invoke<Answered: Decodable>(_ tool: String, input: [String: Any]) async throws -> Answered {
        guard let url = URL(string: "\(credential.origin)/api/v1/tools/\(tool)/invoke") else {
            throw AttendanceAPIFailure.refused(String(localized: "The company address could not be read"))
        }
        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.setValue("Bearer \(credential.token)", forHTTPHeaderField: "Authorization")
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.httpBody = try JSONSerialization.data(withJSONObject: ["input": input])

        let (data, response) = try await URLSession.shared.data(for: request)
        let status = (response as? HTTPURLResponse)?.statusCode ?? 0
        guard (200..<300).contains(status) else {
            throw AttendanceAPIFailure.refused(refusalIn(data) ?? String(localized: "\(tool) answered \(status)"))
        }
        return try JSONDecoder().decode(ToolAnswer<Answered>.self, from: data).result
    }

    private func refusalIn(_ data: Data) -> String? {
        guard let body = try? JSONSerialization.jsonObject(with: data) as? [String: Any] else { return nil }
        return body["error"] as? String
    }
}

private struct ToolAnswer<Result: Decodable>: Decodable {
    let result: Result
}
