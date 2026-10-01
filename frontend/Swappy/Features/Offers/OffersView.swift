import SwiftUI

@MainActor
@Observable
final class OffersModel {
    let api: SwappyAPI
    private(set) var offers: [Offer] = []
    private(set) var hasLoaded = false
    var error: String?

    init(api: SwappyAPI) {
        self.api = api
    }

    func load() async {
        do {
            offers = try await api.offers()
            hasLoaded = true
        } catch {
            self.error = errorMessage(for: error)
        }
    }

    func incoming(_ userId: String) -> [Offer] {
        offers.filter { $0.status.isOpen && $0.isIncoming(for: userId) }
    }

    func outgoing(_ userId: String) -> [Offer] {
        offers.filter { $0.status.isOpen && !$0.isIncoming(for: userId) }
    }

    var history: [Offer] {
        offers.filter { !$0.status.isOpen }
    }
}

struct OffersView: View {
    @Environment(AppSession.self) private var session
    @State private var model: OffersModel

    init(api: SwappyAPI) {
        _model = State(initialValue: OffersModel(api: api))
    }

    var body: some View {
        List {
            section("Eingang", offers: model.incoming(session.userId))
            section("Gesendet", offers: model.outgoing(session.userId))
            section("Verlauf", offers: model.history)
        }
        .overlay {
            if model.hasLoaded && model.offers.isEmpty {
                ContentUnavailableView("Noch keine Angebote", systemImage: "arrow.left.arrow.right",
                                       description: Text("Finde unter „Entdecken“ etwas, das dir gefällt, und biete einen Tausch an."))
            }
        }
        .navigationTitle("Angebote")
        .task {
            await model.load()
            await session.refreshBadges()
        }
        .refreshable {
            await model.load()
            await session.refreshBadges()
        }
        .errorAlert($model.error)
    }

    @ViewBuilder
    private func section(_ title: String, offers: [Offer]) -> some View {
        if !offers.isEmpty {
            Section(title) {
                ForEach(offers) { offer in
                    NavigationLink(value: Route.offer(offer.id)) {
                        OfferRow(offer: offer, userId: session.userId, api: model.api)
                    }
                }
            }
        }
    }
}

struct OfferRow: View {
    let offer: Offer
    let userId: String
    let api: SwappyAPI

    var body: some View {
        HStack(spacing: 12) {
            ItemThumbnail(url: api.imageURL(offer.coverPath), cornerRadius: 8)
                .frame(width: 56, height: 56)
            VStack(alignment: .leading, spacing: 4) {
                Text(headline)
                    .font(.body.weight(.medium))
                    .lineLimit(1)
                Text(summary)
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .lineLimit(2)
            }
            Spacer()
            StatusBadge(text: badgeText, color: offer.status.color)
        }
    }

    private var headline: String {
        let other = offer.counterpartyName(for: userId)
        return offer.isIncoming(for: userId) ? "Von \(other)" : "An \(other)"
    }

    private var summary: String {
        let give = (offer.isIncoming(for: userId) ? offer.requestedItems : offer.offeredItems).map(\.title)
        let get = (offer.isIncoming(for: userId) ? offer.offeredItems : offer.requestedItems).map(\.title)
        return "\(get.joined(separator: ", ")) ⇄ \(give.joined(separator: ", "))"
    }

    private var badgeText: String {
        if offer.status == .accepted && offer.hasConfirmed(userId) { return "Bestätigt" }
        return offer.status.label
    }
}
