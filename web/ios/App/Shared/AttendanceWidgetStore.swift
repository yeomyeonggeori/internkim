import Foundation

struct AttendanceWidgetCredential {
    let token: String
    let tokenName: String
    let origin: String
}

enum AttendanceWidgetStore {
    static let appGroup = "group.app.intern.kim"
    private static let keychainService = "app.intern.kim.widget"
    private static let tokenAccount = "public-api-token"
    private static let installAccount = "install-id"
    private static let tokenNameKey = "widget.tokenName"
    private static let originKey = "widget.origin"

    private static var accessGroup: String? {
        guard let prefix = Bundle.main.object(forInfoDictionaryKey: "AppIdentifierPrefix") as? String,
              !prefix.hasPrefix("$(") else { return nil }
        return "\(prefix)app.intern.kim"
    }

    static var sharedDefaults: UserDefaults? {
        UserDefaults(suiteName: appGroup)
    }

    static func installID() -> String {
        if let held = read(account: installAccount) { return held }
        let made = UUID().uuidString
        write(account: installAccount, value: made)
        return made
    }

    static func heldTokenName() -> String {
        sharedDefaults?.string(forKey: tokenNameKey) ?? ""
    }

    static func credential() -> AttendanceWidgetCredential? {
        guard let token = read(account: tokenAccount),
              let origin = sharedDefaults?.string(forKey: originKey), !origin.isEmpty else { return nil }
        return AttendanceWidgetCredential(token: token, tokenName: heldTokenName(), origin: origin)
    }

    static func keep(_ credential: AttendanceWidgetCredential) {
        write(account: tokenAccount, value: credential.token)
        sharedDefaults?.set(credential.tokenName, forKey: tokenNameKey)
        sharedDefaults?.set(credential.origin, forKey: originKey)
    }

    static func forget() {
        delete(account: tokenAccount)
        sharedDefaults?.removeObject(forKey: tokenNameKey)
        sharedDefaults?.removeObject(forKey: originKey)
    }

    private static func query(account: String) -> [String: Any] {
        var query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: keychainService,
            kSecAttrAccount as String: account,
        ]
        if let accessGroup { query[kSecAttrAccessGroup as String] = accessGroup }
        return query
    }

    private static func read(account: String) -> String? {
        var query = query(account: account)
        query[kSecReturnData as String] = true
        query[kSecMatchLimit as String] = kSecMatchLimitOne
        var found: CFTypeRef?
        guard SecItemCopyMatching(query as CFDictionary, &found) == errSecSuccess,
              let data = found as? Data else { return nil }
        return String(data: data, encoding: .utf8)
    }

    private static func write(account: String, value: String) {
        delete(account: account)
        var query = query(account: account)
        query[kSecValueData as String] = Data(value.utf8)
        query[kSecAttrAccessible as String] = kSecAttrAccessibleAfterFirstUnlock
        SecItemAdd(query as CFDictionary, nil)
    }

    private static func delete(account: String) {
        SecItemDelete(query(account: account) as CFDictionary)
    }
}
