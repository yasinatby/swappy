import SwiftUI

@MainActor
@Observable
final class OfferDetailModel {
    let api: SwappyAPI
    let offerId: String
    private(set) var offer: Offer?
    private(set) var isWorking = false
    private(set) var notFound = false
    var error: String?

    init(api: SwappyAPI, offerId: String) {
        self.api = api
        self.offerId = offerId
    }

    func load() async {
        do {
            offer = try await api.offer(offerId)
        } catch APIError.server(404, _, _) {
            notFound = true
        } catch {
            self.error = errorMessage(for: error)
        }
    }

    func perform(_ action: OfferAction) async {
        isWorking = true
        defer { isWorking = false }
        do {
            offer = try await api.perform(action, offerId: offerId)
        } catch {
            self.error = errorMessage(for: error)
            await load() // state may have moved on (e.g. cancelled by a competing acceptance)
        }
    }
}

struct OfferDetailView: View {
    @Environment(AppSession.self) private var session
    @State private var model: OfferDetailModel
    @State private var pendingAction: OfferAction?
    @State private var showReport = false

    init(api: SwappyAPI, offerId: String) {
        _model = State(initialValue: OfferDetailModel(api: api, offerId: offerId))
    }

    var body: some View {
        Group {
            if let offer = model.offer {
                content(offer)
            } else if model.notFound {
                ContentUnavailableView("Angebot nicht gefunden", systemImage: "questionmark.folder")
            } else {
                ProgressView()
            }
        }
        .navigationTitle("Tauschangebot")
        .navigationBarTitleDisplayMode(.inline)
        .task { await model.load() }
        .refreshable { await model.load() }
        .errorAlert($model.error)
    }

    private var userId: String { session.userId }

    @ViewBuilder
    private func content(_ offer: Offer) -> some View {
        let incoming = offer.isIncoming(for: userId)
        let other = offer.counterpartyName(for: userId)
        List {
            Section {
                HStack {
                    StatusBadge(text: offer.status.label, color: offer.status.color)
                    Spacer()
                    Text(statusDetail(offer)).font(.caption).foregroundStyle(.secondary)
                }
                if let message = offer.message {
                    Text("„\(message)“").italic()
                }
            }

            Section(incoming ? "\(other) bietet dir" : "Du bietest") {
                itemRows(offer.offeredItems)
            }
            Section(incoming ? "Für deine Sachen" : "Für die Sachen von \(other)") {
                itemRows(offer.requestedItems)
            }

            if offer.status == .accepted {
                Section {
                    confirmationRow(name: "Du", confirmed: offer.hasConfirmed(userId))
                    confirmationRow(name: other, confirmed: offer.hasConfirmed(offer.counterpartyId(for: userId)))
                } header: {
                    Text("Übergabe")
                } footer: {
                    Text("Trefft euch und tauscht. Danach bestätigt ihr beide die Übergabe – dann ist der Tausch abgeschlossen.")
                }
            }

            Section {
                NavigationLink(value: Route.profile(offer.counterpartyId(for: userId))) {
                    Label("Profil von \(other)", systemImage: "person.crop.circle")
                }
                Button("\(other) melden", systemImage: "flag") { showReport = true }
                    .foregroundStyle(.red)
            }
        }
        .safeAreaInset(edge: .bottom) { actions(offer) }
        .sheet(isPresented: $showReport) {
            ReportSheet(api: model.api, subject: .profile, subjectId: offer.counterpartyId(for: userId))
        }
        .confirmationDialog(confirmTitle, isPresented: Binding(
            get: { pendingAction != nil }, set: { if !$0 { pendingAction = nil } }
        ), titleVisibility: .visible, presenting: pendingAction) { action in
            Button(confirmButton(action), role: action == .accept || action == .confirm ? nil : .destructive) {
                Task {
                    await model.perform(action)
                    await session.refreshBadges()
                }
            }
        } message: { action in
            Text(confirmMessage(action))
        }
    }

    private func itemRows(_ items: [OfferItem]) -> some View {
        ForEach(items) { item in
            NavigationLink(value: Route.item(item.id)) {
                HStack(spacing: 12) {
                    ItemThumbnail(url: model.api.imageURL(item.coverPath), cornerRadius: 8)
                        .frame(width: 48, height: 48)
                    Text(item.title)
                    Spacer()
                    if item.status != .active {
                        StatusBadge(text: item.status.label, color: item.status.color)
                    }
                }
            }
        }
    }

    private func confirmationRow(name: String, confirmed: Bool) -> some View {
        Label {
            Text(confirmed ? "\(name): Übergabe bestätigt" : "\(name): noch offen")
        } icon: {
            Image(systemName: confirmed ? "checkmark.circle.fill" : "circle")
                .foregroundStyle(confirmed ? .green : .secondary)
        }
    }

    @ViewBuilder
    private func actions(_ offer: Offer) -> some View {
        let incoming = offer.isIncoming(for: userId)
        let buttons: [(OfferAction, String, String, Bool)] = switch offer.status {
        case .pending where incoming:
            [(.decline, "Ablehnen", "xmark", false), (.accept, "Annehmen", "checkmark", true)]
        case .pending:
            [(.cancel, "Angebot zurückziehen", "arrow.uturn.backward", false)]
        case .accepted where !offer.hasConfirmed(userId):
            [(.cancel, "Absagen", "xmark", false), (.confirm, "Übergabe bestätigen", "hand.thumbsup", true)]
        case .accepted:
            [(.cancel, "Tausch absagen", "xmark", false)]
        default:
            []
        }
        if !buttons.isEmpty {
            HStack(spacing: 12) {
                ForEach(buttons, id: \.0) { action, title, symbol, prominent in
                    Button {
                        pendingAction = action
                    } label: {
                        Label(title, systemImage: symbol).frame(maxWidth: .infinity)
                    }
                    .controlSize(.large)
                    .modifier(ProminenceModifier(prominent: prominent))
                    .accessibilityIdentifier("offerAction-\(action.rawValue)")
                }
            }
            .disabled(model.isWorking)
            .padding()
            .background(.bar)
        }
    }

    private func statusDetail(_ offer: Offer) -> String {
        switch offer.status {
        case .pending: expiryText(offer.expiresAt)
        case .accepted: "angenommen \(offer.acceptedAt?.relativeGerman ?? "")"
        default: offer.createdAt.shortGerman
        }
    }

    private var confirmTitle: String {
        switch pendingAction {
        case .accept: "Angebot annehmen?"
        case .decline: "Angebot ablehnen?"
        case .cancel: "Wirklich stornieren?"
        case .confirm: "Übergabe bestätigen?"
        case nil: ""
        }
    }

    private func confirmButton(_ action: OfferAction) -> String {
        switch action {
        case .accept: "Annehmen"
        case .decline: "Ablehnen"
        case .cancel: "Stornieren"
        case .confirm: "Ja, getauscht"
        }
    }

    private func confirmMessage(_ action: OfferAction) -> String {
        switch action {
        case .accept: "Alle beteiligten Gegenstände werden reserviert. Andere offene Angebote dafür werden automatisch storniert."
        case .decline: "Die Gegenseite wird benachrichtigt."
        case .cancel: "Reservierte Gegenstände werden wieder freigegeben."
        case .confirm: "Bestätige erst, wenn du die Sachen wirklich getauscht hast."
        }
    }
}

private struct ProminenceModifier: ViewModifier {
    let prominent: Bool

    func body(content: Content) -> some View {
        if prominent {
            content.buttonStyle(.borderedProminent)
        } else {
            content.buttonStyle(.bordered)
        }
    }
}
