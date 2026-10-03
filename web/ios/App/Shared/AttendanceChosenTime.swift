import Foundation

struct AttendanceChosenTime: Codable, Equatable {
    let day: String
    let time: String

    private static let key = "widget.chosenTime"
    private static let choosingScheme = "internkim-attendance"
    private static let choosingHost = "choose-time"
    private static let kindQuery = "kind"

    init(day: String, time: String) {
        self.day = day
        self.time = time
    }

    init?(day: String?, time: String?) {
        guard let day, let time else { return nil }
        self.init(day: day, time: time)
    }

    static func of(_ moment: Date, in timeZone: TimeZone) -> AttendanceChosenTime {
        AttendanceChosenTime(day: CompanyClock.day(of: moment, in: timeZone), time: CompanyClock.time(of: moment, in: timeZone))
    }

    static func held(today: String, in store: UserDefaults? = AttendanceWidgetStore.sharedDefaults) -> AttendanceChosenTime? {
        guard let data = store?.data(forKey: key),
              let chosen = try? JSONDecoder().decode(AttendanceChosenTime.self, from: data),
              chosen.day == today else { return nil }
        return chosen
    }

    static func keep(_ chosen: AttendanceChosenTime, in store: UserDefaults? = AttendanceWidgetStore.sharedDefaults) {
        guard let encoded = try? JSONEncoder().encode(chosen) else { return }
        store?.set(encoded, forKey: key)
    }

    static func clear(in store: UserDefaults? = AttendanceWidgetStore.sharedDefaults) {
        store?.removeObject(forKey: key)
    }

    static func choosingURL(kind: String) -> URL? {
        var components = URLComponents()
        components.scheme = choosingScheme
        components.host = choosingHost
        components.queryItems = [URLQueryItem(name: kindQuery, value: kind)]
        return components.url
    }

    static func kind(choosingFrom url: URL) -> String? {
        guard url.scheme == choosingScheme, url.host == choosingHost else { return nil }
        return URLComponents(url: url, resolvingAgainstBaseURL: false)?
            .queryItems?
            .first(where: { $0.name == kindQuery })?
            .value
    }

    func moment(in timeZone: TimeZone) -> Date? {
        let formatter = DateFormatter()
        formatter.calendar = Calendar(identifier: .iso8601)
        formatter.locale = Locale(identifier: "en_US_POSIX")
        formatter.timeZone = timeZone
        formatter.dateFormat = "yyyy-MM-dd HH:mm"
        return formatter.date(from: "\(day) \(time)")
    }
}
