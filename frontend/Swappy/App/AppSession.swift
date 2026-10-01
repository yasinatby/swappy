import Foundation
import Observation

/// Who is logged in and against which server. Owns the API client.
@MainActor
@Observable
final class AppSession {
    enum State: Equatable {
        case restoring
        case loggedOut
        case loggedIn
    }

    private static let tokenKey = "session-token"
    private static let serverKey = "server-url"
    static let defaultServer = "http://localhost:8080"

    private(set) var state: State = .restoring
    private(set) var currentUser: Profile?
    private(set) var api: SwappyAPI
    private(set) var serverURL: String

    /// Badge counts for the tab bar.
    var openIncomingOffers = 0
    var unreadNotifications = 0

    init() {
        // UI tests start from the login screen every time.
        if ProcessInfo.processInfo.arguments.contains("-resetSession") {
            Keychain.set(nil, for: Self.tokenKey)
        }
        let server = UserDefaults.standard.string(forKey: Self.serverKey) ?? Self.defaultServer
        serverURL = server
        let client = APIClient(baseURL: URL(string: server) ?? URL(string: Self.defaultServer)!,
                               token: Keychain.get(Self.tokenKey))
        api = SwappyAPI(client: client)
        client.onUnauthorized = { [weak self] in
            Task { @MainActor in self?.logout() }
        }
    }

    var userId: String { currentUser?.id ?? "" }

    func restore() async {
        guard api.client.token != nil else {
            state = .loggedOut
            return
        }
        do {
            currentUser = try await api.me()
            state = .loggedIn
            await refreshBadges()
        } catch {
            // Server unreachable or token invalid: back to the login screen,
            // which shows the server address and lets the dev retry.
            state = .loggedOut
        }
    }

    /// Changes the backend address (e.g. the Mac's LAN IP for a real device).
    func setServer(_ address: String) -> Bool {
        let trimmed = address.trimmingCharacters(in: .whitespacesAndNewlines)
        guard let url = URL(string: trimmed), url.scheme?.hasPrefix("http") == true, url.host() != nil else {
            return false
        }
        serverURL = trimmed
        UserDefaults.standard.set(trimmed, forKey: Self.serverKey)
        api.client.baseURL = url
        return true
    }

    func devLogin(displayName: String) async throws {
        let response = try await api.devLogin(displayName: displayName)
        api.client.token = response.token
        Keychain.set(response.token, for: Self.tokenKey)
        currentUser = response.profile
        state = .loggedIn
        await refreshBadges()
    }

    func logout() {
        guard state != .loggedOut else { return }
        api.client.token = nil
        Keychain.set(nil, for: Self.tokenKey)
        currentUser = nil
        openIncomingOffers = 0
        unreadNotifications = 0
        state = .loggedOut
    }

    func updateCurrentUser(_ profile: Profile) {
        currentUser = profile
    }

    func refreshBadges() async {
        guard state == .loggedIn else { return }
        async let offers = try? api.offers()
        async let notifications = try? api.notifications()
        let userId = userId
        openIncomingOffers = (await offers ?? []).filter { $0.status == .pending && $0.responderId == userId }.count
        unreadNotifications = (await notifications ?? []).filter { $0.readAt == nil }.count
    }
}
