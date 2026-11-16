import AppIntents

struct AttendanceWorkplaceEntity: AppEntity {
    let name: String

    var id: String { name }

    static let typeDisplayRepresentation = TypeDisplayRepresentation(name: "Work location")
    static let defaultQuery = AttendanceWorkplaceQuery()

    var displayRepresentation: DisplayRepresentation {
        DisplayRepresentation(title: "\(name)")
    }
}

struct AttendanceWorkplaceQuery: EntityQuery {
    func entities(for identifiers: [String]) async throws -> [AttendanceWorkplaceEntity] {
        identifiers.map { AttendanceWorkplaceEntity(name: $0) }
    }

    func suggestedEntities() async throws -> [AttendanceWorkplaceEntity] {
        try await AttendanceAPI.held().settings().workLocations.map { AttendanceWorkplaceEntity(name: $0.name) }
    }

    func defaultResult() async -> AttendanceWorkplaceEntity? {
        try? await suggestedEntities().first
    }
}

struct AttendanceWidgetConfiguration: WidgetConfigurationIntent {
    static let title: LocalizedStringResource = "Attendance"
    static let description = IntentDescription("See today's hours, and clock in and out with a tap.")

    @Parameter(title: "Default work location")
    var workplace: AttendanceWorkplaceEntity?
}
