import SwiftUI

@main
struct SwappyApp: App {
    @State private var session = AppSession()

    var body: some Scene {
        WindowGroup {
            RootView()
                .environment(session)
                .tint(.swappyAccent)
        }
    }
}

extension Color {
    static let swappyAccent = Color("AccentColor")
}
