import SwiftUI

@MainActor
@Observable
final class OfferComposerModel {
    let api: SwappyAPI
    let target: Item
    private(set) var myItems: [Item] = []
    /// Further active items of the same owner that can be requested as well.
    private(set) var ownerItems: [Item] = []
    private(set) var hasLoaded = false
    private(set) var isSending = false
    var offered: Set<String> = []
    var requested: Set<String> = []
    var message = ""
    var error: String?

    init(api: SwappyAPI, target: Item) {
        self.api = api
        self.target = target
    }

    func load() async {
        do {
            async let mine = api.myItems()
            async let theirs = api.feed(ownerId: target.ownerId)
            myItems = try await mine.filter { $0.status == .active }
            ownerItems = try await theirs.filter { $0.id != target.id }
            hasLoaded = true
        } catch {
            self.error = errorMessage(for: error)
        }
    }

    var canSend: Bool { !offered.isEmpty && !isSending }

    func send() async -> Offer? {
        isSending = true
        defer { isSending = false }
        let trimmed = message.trimmingCharacters(in: .whitespacesAndNewlines)
        do {
            return try await api.createOffer(CreateOfferInput(
                targetItemId: target.id,
                offeredItemIds: Array(offered),
                requestedItemIds: Array(requested),
                message: trimmed.isEmpty ? nil : trimmed))
        } catch {
            self.error = errorMessage(for: error)
            return nil
        }
    }
}

struct OfferComposerView: View {
    @Environment(AppSession.self) private var session
    @Environment(\.dismiss) private var dismiss
    @State private var model: OfferComposerModel
    @State private var showEditor = false
    @State private var sentOffer: Offer?

    init(api: SwappyAPI, target: Item) {
        _model = State(initialValue: OfferComposerModel(api: api, target: target))
    }

    var body: some View {
        NavigationStack {
            Group {
                if !model.hasLoaded {
                    ProgressView()
                } else if model.myItems.isEmpty {
                    // Invariant 4: also enforced by the backend.
                    ContentUnavailableView {
                        Label("Erst etwas einstellen", systemImage: "shippingbox")
                    } description: {
                        Text("Um ein Tauschangebot zu machen, brauchst du mindestens einen aktiven Gegenstand.")
                    } actions: {
                        Button("Gegenstand einstellen") { showEditor = true }
                            .buttonStyle(.borderedProminent)
                    }
                } else if let sentOffer {
                    ContentUnavailableView {
                        Label("Angebot gesendet", systemImage: "paperplane.circle.fill")
                    } description: {
                        Text("\(sentOffer.responderName) wird benachrichtigt. Du findest das Angebot unter „Angebote“.")
                    } actions: {
                        Button("Fertig") { dismiss() }.buttonStyle(.borderedProminent)
                    }
                } else {
                    form
                }
            }
            .navigationTitle("Tausch anbieten")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    if sentOffer == nil { Button("Abbrechen") { dismiss() } }
                }
                ToolbarItem(placement: .confirmationAction) {
                    if model.hasLoaded && !model.myItems.isEmpty && sentOffer == nil {
                        Button("Senden") {
                            Task {
                                sentOffer = await model.send()
                                await session.refreshBadges()
                            }
                        }
                        .disabled(!model.canSend)
                        .accessibilityIdentifier("sendOfferButton")
                    }
                }
            }
            .sheet(isPresented: $showEditor, onDismiss: { Task { await model.load() } }) {
                ItemEditorView(api: model.api)
            }
            .task { await model.load() }
            .errorAlert($model.error)
        }
    }

    private var form: some View {
        Form {
            Section {
                HStack(spacing: 12) {
                    ItemThumbnail(url: model.api.imageURL(model.target.coverPath), category: model.target.category, cornerRadius: 8)
                        .frame(width: 48, height: 48)
                    VStack(alignment: .leading) {
                        Text(model.target.title).font(.headline)
                        Text("von \(model.target.ownerName)").font(.caption).foregroundStyle(.secondary)
                    }
                }
                if let wanted = model.target.wantedText {
                    Label("Sucht: \(wanted)", systemImage: "arrow.left.arrow.right")
                        .font(.subheadline)
                        .foregroundStyle(Color.swappyAccent)
                }
            } header: {
                Text("Du möchtest")
            }

            if !model.ownerItems.isEmpty {
                Section {
                    selectableRows(model.ownerItems, selection: $model.requested)
                } header: {
                    Text("Zusätzlich von \(model.target.ownerName)")
                } footer: {
                    Text("Optional – du kannst weitere Sachen in denselben Tausch nehmen.")
                }
            }

            Section {
                selectableRows(model.myItems, selection: $model.offered)
            } header: {
                Text("Du bietest dafür")
            } footer: {
                Text("Wähle mindestens einen deiner Gegenstände.")
            }

            Section("Nachricht (optional)") {
                TextField("Hallo! Hätte Interesse an …", text: $model.message, axis: .vertical)
                    .lineLimit(2 ... 5)
            }
        }
    }

    private func selectableRows(_ items: [Item], selection: Binding<Set<String>>) -> some View {
        ForEach(items) { item in
            Button {
                if selection.wrappedValue.contains(item.id) {
                    selection.wrappedValue.remove(item.id)
                } else {
                    selection.wrappedValue.insert(item.id)
                }
            } label: {
                HStack(spacing: 12) {
                    Image(systemName: selection.wrappedValue.contains(item.id) ? "checkmark.circle.fill" : "circle")
                        .font(.title3)
                        .foregroundStyle(selection.wrappedValue.contains(item.id) ? Color.swappyAccent : .secondary)
                    ItemThumbnail(url: model.api.imageURL(item.coverPath), category: item.category, cornerRadius: 6)
                        .frame(width: 40, height: 40)
                    Text(item.title).foregroundStyle(.primary)
                }
            }
            .accessibilityIdentifier("select-\(item.title)")
        }
    }
}
