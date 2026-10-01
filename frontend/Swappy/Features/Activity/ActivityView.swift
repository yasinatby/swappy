import SwiftUI

/// In-app list of trade events. Push via APNs comes later and will read the
/// same notifications.
struct ActivityView: View {
    @Environment(AppSession.self) private var session
    let api: SwappyAPI

    @State private var notifications: [AppNotification] = []
    @State private var hasLoaded = false
    @State private var error: String?

    var body: some View {
        List(notifications) { notification in
            Group {
                if let offerId = notification.offerId {
                    NavigationLink(value: Route.offer(offerId)) { row(notification) }
                } else {
                    row(notification)
                }
            }
            .listRowBackground(notification.readAt == nil ? Color.swappyAccent.opacity(0.08) : nil)
        }
        .overlay {
            if hasLoaded && notifications.isEmpty {
                ContentUnavailableView("Noch nichts passiert", systemImage: "bell",
                                       description: Text("Hier erscheinen neue Angebote und Neuigkeiten zu deinen Tauschen."))
            }
        }
        .navigationTitle("Aktivität")
        .task { await load() }
        .refreshable { await load() }
        .errorAlert($error)
    }

    private func row(_ notification: AppNotification) -> some View {
        HStack(spacing: 12) {
            Image(systemName: notification.symbol)
                .font(.title3)
                .foregroundStyle(Color.swappyAccent)
                .frame(width: 28)
            VStack(alignment: .leading, spacing: 2) {
                Text(notification.text)
                Text(notification.createdAt.relativeGerman)
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
        }
    }

    private func load() async {
        do {
            notifications = try await api.notifications()
            hasLoaded = true
            if notifications.contains(where: { $0.readAt == nil }) {
                try await api.markNotificationsRead()
                await session.refreshBadges()
            }
        } catch {
            self.error = errorMessage(for: error)
        }
    }
}
