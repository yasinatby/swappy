import SwiftUI

/// Navigation targets shared by all tabs.
enum Route: Hashable {
    case item(String)
    case offer(String)
    case profile(String)
    case activity
    case blockedUsers
}

private struct RouteDestination: View {
    @Environment(AppSession.self) private var session
    let route: Route

    var body: some View {
        switch route {
        case let .item(id): ItemDetailView(api: session.api, itemId: id)
        case let .offer(id): OfferDetailView(api: session.api, offerId: id)
        case let .profile(id): PublicProfileView(api: session.api, profileId: id)
        case .activity: ActivityView(api: session.api)
        case .blockedUsers: BlockedUsersView(api: session.api)
        }
    }
}

extension View {
    func withRoutes() -> some View {
        navigationDestination(for: Route.self) { RouteDestination(route: $0) }
    }
}
