import Foundation

enum AttendanceDefaultWorkplace: Equatable {
    case none
    case chosen(WorkLocation)
    case gone(String)

    static func of(configured: String?, among locations: [WorkLocation]) -> AttendanceDefaultWorkplace {
        guard let configured, !configured.isEmpty else {
            guard let first = locations.first else { return .none }
            return .chosen(first)
        }
        guard let registered = locations.first(where: { $0.name == configured }) else {
            return .gone(configured)
        }
        return .chosen(registered)
    }

    static func others(besides chosen: WorkLocation?, among locations: [WorkLocation]) -> [WorkLocation] {
        guard let chosen else { return locations }
        return locations.filter { $0.name != chosen.name }
    }

    var location: WorkLocation? {
        guard case let .chosen(location) = self else { return nil }
        return location
    }

    var needsChoice: Bool {
        if case .gone = self { return true }
        return false
    }
}
