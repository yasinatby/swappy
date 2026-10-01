import SwiftUI

@MainActor
@Observable
final class ItemDetailModel {
    let api: SwappyAPI
    let itemId: String
    private(set) var item: Item?
    private(set) var isWorking = false
    private(set) var notFound = false
    var error: String?

    init(api: SwappyAPI, itemId: String) {
        self.api = api
        self.itemId = itemId
    }

    func load() async {
        do {
            item = try await api.item(itemId)
        } catch APIError.server(404, _, _) {
            notFound = true
        } catch {
            self.error = errorMessage(for: error)
        }
    }

    func publish() async {
        await run { self.item = try await self.api.publishItem(self.itemId) }
    }

    func delete() async -> Bool {
        await run { try await self.api.deleteItem(self.itemId) }
    }

    func block(_ userId: String) async -> Bool {
        await run { try await self.api.block(userId) }
    }

    @discardableResult
    private func run(_ work: () async throws -> Void) async -> Bool {
        isWorking = true
        defer { isWorking = false }
        do {
            try await work()
            return true
        } catch {
            self.error = errorMessage(for: error)
            return false
        }
    }
}

struct ItemDetailView: View {
    @Environment(AppSession.self) private var session
    @Environment(\.dismiss) private var dismiss
    @State private var model: ItemDetailModel
    @State private var showEditor = false
    @State private var showComposer = false
    @State private var showReport = false
    @State private var confirmDelete = false
    @State private var confirmBlock = false

    init(api: SwappyAPI, itemId: String) {
        _model = State(initialValue: ItemDetailModel(api: api, itemId: itemId))
    }

    var body: some View {
        Group {
            if let item = model.item {
                content(item)
            } else if model.notFound {
                ContentUnavailableView("Nicht mehr verfügbar", systemImage: "eye.slash",
                                       description: Text("Dieser Gegenstand wurde entfernt oder ist nicht sichtbar."))
            } else {
                ProgressView()
            }
        }
        .navigationBarTitleDisplayMode(.inline)
        .task { await model.load() }
        .refreshable { await model.load() }
        .errorAlert($model.error)
    }

    private func isMine(_ item: Item) -> Bool { item.ownerId == session.userId }

    @ViewBuilder
    private func content(_ item: Item) -> some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 16) {
                gallery(item)
                VStack(alignment: .leading, spacing: 8) {
                    HStack {
                        Label(Category.label(for: item.category), systemImage: Category.symbol(for: item.category))
                        Text("·")
                        Text(item.condition.label)
                        Spacer()
                        if isMine(item) || item.status != .active {
                            StatusBadge(text: item.status.label, color: item.status.color)
                        }
                    }
                    .font(.subheadline)
                    .foregroundStyle(.secondary)
                    Text(item.title)
                        .font(.title2.bold())
                }

                if let wanted = item.wantedText {
                    VStack(alignment: .leading, spacing: 4) {
                        Label("Das suche ich dafür", systemImage: "arrow.left.arrow.right")
                            .font(.caption.weight(.semibold))
                            .foregroundStyle(Color.swappyAccent)
                        Text(wanted)
                    }
                    .padding()
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .background(Color.swappyAccent.opacity(0.1), in: RoundedRectangle(cornerRadius: 12))
                }

                if let description = item.description {
                    Text(description)
                }

                if !isMine(item) {
                    NavigationLink(value: Route.profile(item.ownerId)) {
                        HStack {
                            Image(systemName: "person.crop.circle.fill")
                                .font(.largeTitle)
                                .foregroundStyle(.secondary)
                            VStack(alignment: .leading) {
                                Text(item.ownerName).font(.headline)
                                if let distance = distanceText(item.distanceKm) {
                                    Label(distance, systemImage: "location").font(.caption).foregroundStyle(.secondary)
                                }
                            }
                            Spacer()
                            Image(systemName: "chevron.right").foregroundStyle(.tertiary)
                        }
                        .padding()
                        .background(Color(.secondarySystemBackground), in: RoundedRectangle(cornerRadius: 12))
                    }
                    .buttonStyle(.plain)
                }
            }
            .padding()
        }
        .safeAreaInset(edge: .bottom) { actionBar(item) }
        .toolbar { toolbar(item) }
        .sheet(isPresented: $showEditor, onDismiss: { Task { await model.load() } }) {
            ItemEditorView(api: model.api, item: item)
        }
        .sheet(isPresented: $showComposer) {
            OfferComposerView(api: model.api, target: item)
        }
        .sheet(isPresented: $showReport) {
            ReportSheet(api: model.api, subject: .item, subjectId: item.id)
        }
        .confirmationDialog("Gegenstand löschen?", isPresented: $confirmDelete, titleVisibility: .visible) {
            Button("Löschen", role: .destructive) {
                Task { if await model.delete() { dismiss() } }
            }
        } message: {
            Text("Offene Angebote für diesen Gegenstand werden storniert.")
        }
        .confirmationDialog("\(item.ownerName) blockieren?", isPresented: $confirmBlock, titleVisibility: .visible) {
            Button("Blockieren", role: .destructive) {
                Task { if await model.block(item.ownerId) { dismiss() } }
            }
        } message: {
            Text("Ihr seht gegenseitig keine Gegenstände mehr und offene Angebote zwischen euch werden storniert.")
        }
    }

    private func gallery(_ item: Item) -> some View {
        let images = item.images ?? []
        return Group {
            if images.isEmpty {
                ItemThumbnail(url: nil, category: item.category, cornerRadius: 16)
            } else {
                TabView {
                    ForEach(images) { image in
                        ItemThumbnail(url: model.api.imageURL(image.path), cornerRadius: 16)
                    }
                }
                .tabViewStyle(.page(indexDisplayMode: images.count > 1 ? .always : .never))
            }
        }
        .aspectRatio(1, contentMode: .fit)
    }

    @ViewBuilder
    private func actionBar(_ item: Item) -> some View {
        Group {
            if isMine(item) {
                if item.status == .draft {
                    Button {
                        Task { await model.publish() }
                    } label: {
                        Label("Veröffentlichen", systemImage: "paperplane.fill").frame(maxWidth: .infinity)
                    }
                    .buttonStyle(.borderedProminent)
                    .controlSize(.large)
                } else if item.status == .reserved {
                    Label("Steckt in einem angenommenen Tausch", systemImage: "lock.fill")
                        .font(.subheadline)
                        .foregroundStyle(.secondary)
                }
            } else if item.status == .active {
                Button {
                    showComposer = true
                } label: {
                    Label("Tausch anbieten", systemImage: "arrow.left.arrow.right").frame(maxWidth: .infinity)
                }
                .buttonStyle(.borderedProminent)
                .controlSize(.large)
                .accessibilityIdentifier("offerTradeButton")
            }
        }
        .disabled(model.isWorking)
        .padding()
        .background(.bar)
    }

    @ToolbarContentBuilder
    private func toolbar(_ item: Item) -> some ToolbarContent {
        ToolbarItem(placement: .topBarTrailing) {
            Menu {
                if isMine(item) {
                    if item.status.isEditable {
                        Button("Bearbeiten", systemImage: "pencil") { showEditor = true }
                        Button("Löschen", systemImage: "trash", role: .destructive) { confirmDelete = true }
                    }
                } else {
                    Button("Gegenstand melden", systemImage: "flag") { showReport = true }
                    Button("\(item.ownerName) blockieren", systemImage: "hand.raised", role: .destructive) { confirmBlock = true }
                }
            } label: {
                Image(systemName: "ellipsis.circle")
            }
            .disabled(isMine(item) && !item.status.isEditable)
        }
    }
}
