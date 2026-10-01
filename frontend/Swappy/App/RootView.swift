import SwiftUI

struct RootView: View {
    @Environment(AppSession.self) private var session

    var body: some View {
        Group {
            switch session.state {
            case .restoring:
                ProgressView("Swappy wird gestartet …")
            case .loggedOut:
                DevLoginView()
            case .loggedIn:
                MainTabView()
            }
        }
        .animation(.default, value: session.state)
        .task { await session.restore() }
    }
}

struct MainTabView: View {
    @Environment(AppSession.self) private var session
    @Environment(\.scenePhase) private var scenePhase

    var body: some View {
        TabView {
            NavigationStack { FeedView(api: session.api).withRoutes() }
                .tabItem { Label("Entdecken", systemImage: "magnifyingglass") }
            NavigationStack { MyItemsView(api: session.api).withRoutes() }
                .tabItem { Label("Meine Sachen", systemImage: "shippingbox") }
            NavigationStack { OffersView(api: session.api).withRoutes() }
                .tabItem { Label("Angebote", systemImage: "arrow.left.arrow.right") }
                .badge(session.openIncomingOffers)
            NavigationStack { ProfileView(api: session.api).withRoutes() }
                .tabItem { Label("Profil", systemImage: "person.crop.circle") }
        }
        .onChange(of: scenePhase) { _, phase in
            if phase == .active { Task { await session.refreshBadges() } }
        }
    }
}
