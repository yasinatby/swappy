import SwiftUI

@MainActor
@Observable
final class MyItemsModel {
    let api: SwappyAPI
    private(set) var items: [Item] = []
    private(set) var hasLoaded = false
    var error: String?

    init(api: SwappyAPI) {
        self.api = api
    }

    func load() async {
        do {
            items = try await api.myItems()
            hasLoaded = true
        } catch {
            self.error = errorMessage(for: error)
        }
    }

    func delete(_ item: Item) async {
        do {
            try await api.deleteItem(item.id)
            items.removeAll { $0.id == item.id }
        } catch {
            self.error = errorMessage(for: error)
        }
    }

    /// Sections in the order a user cares about them.
    var sections: [(title: String, items: [Item])] {
        let groups: [(String, [ItemStatus])] = [
            ("Aktiv", [.active]),
            ("Im Tausch", [.reserved]),
            ("Entwürfe", [.draft]),
            ("Getauscht", [.traded]),
            ("Von der Moderation entfernt", [.removed]),
        ]
        return groups.compactMap { title, statuses in
            let matching = items.filter { statuses.contains($0.status) }
            return matching.isEmpty ? nil : (title, matching)
        }
    }
}

struct MyItemsView: View {
    @State private var model: MyItemsModel
    @State private var showEditor = false

    init(api: SwappyAPI) {
        _model = State(initialValue: MyItemsModel(api: api))
    }

    var body: some View {
        List {
            if model.hasLoaded && !model.items.contains(where: { $0.status == .active }) {
                Section {
                    Label {
                        Text("Stell mindestens einen Gegenstand aktiv ein, um selbst Tauschangebote machen zu können.")
                    } icon: {
                        Image(systemName: "info.circle.fill").foregroundStyle(Color.swappyAccent)
                    }
                    .font(.subheadline)
                }
            }
            ForEach(model.sections, id: \.title) { section in
                Section(section.title) {
                    ForEach(section.items) { item in
                        NavigationLink(value: Route.item(item.id)) {
                            ItemRow(item: item, imageURL: model.api.imageURL(item.coverPath))
                        }
                        .swipeActions {
                            if item.status.isEditable {
                                Button("Löschen", systemImage: "trash", role: .destructive) {
                                    Task { await model.delete(item) }
                                }
                            }
                        }
                    }
                }
            }
        }
        .overlay {
            if model.hasLoaded && model.items.isEmpty {
                ContentUnavailableView {
                    Label("Noch nichts eingestellt", systemImage: "shippingbox")
                } description: {
                    Text("Was liegt bei dir herum, das jemand anderes gebrauchen kann?")
                } actions: {
                    Button("Gegenstand einstellen") { showEditor = true }
                        .buttonStyle(.borderedProminent)
                }
            }
        }
        .navigationTitle("Meine Sachen")
        .toolbar {
            ToolbarItem(placement: .topBarTrailing) {
                Button {
                    showEditor = true
                } label: {
                    Image(systemName: "plus.circle.fill").font(.title3)
                }
                .accessibilityLabel("Gegenstand einstellen")
                .accessibilityIdentifier("addItemButton")
            }
        }
        .sheet(isPresented: $showEditor, onDismiss: { Task { await model.load() } }) {
            ItemEditorView(api: model.api)
        }
        .task { await model.load() }
        .refreshable { await model.load() }
        .errorAlert($model.error)
    }
}
