import SwiftUI

@MainActor
@Observable
final class FeedModel {
    let api: SwappyAPI
    private(set) var items: [Item] = []
    private(set) var isLoading = false
    private(set) var canLoadMore = true
    var search = ""
    var category: String?
    var error: String?

    init(api: SwappyAPI) {
        self.api = api
    }

    func reload() async {
        isLoading = true
        defer { isLoading = false }
        do {
            let page = try await api.feed(search: search, category: category)
            items = page
            canLoadMore = page.count == 50
        } catch {
            self.error = errorMessage(for: error)
        }
    }

    func loadMoreIfNeeded(after item: Item) async {
        guard canLoadMore, !isLoading, item.id == items.last?.id else { return }
        isLoading = true
        defer { isLoading = false }
        do {
            let page = try await api.feed(search: search, category: category, offset: items.count)
            items += page.filter { new in !items.contains { $0.id == new.id } }
            canLoadMore = page.count == 50
        } catch {
            self.error = errorMessage(for: error)
        }
    }
}

struct FeedView: View {
    @Environment(AppSession.self) private var session
    @State private var model: FeedModel

    init(api: SwappyAPI) {
        _model = State(initialValue: FeedModel(api: api))
    }

    private let columns = [GridItem(.flexible(), spacing: 12), GridItem(.flexible(), spacing: 12)]

    var body: some View {
        ScrollView {
            categoryChips
            if model.items.isEmpty && !model.isLoading {
                ContentUnavailableView {
                    Label("Nichts gefunden", systemImage: "magnifyingglass")
                } description: {
                    Text(model.search.isEmpty && model.category == nil
                        ? "Noch hat niemand etwas zum Tauschen eingestellt."
                        : "Versuch es mit einem anderen Suchbegriff oder einer anderen Kategorie.")
                }
                .padding(.top, 60)
            } else {
                LazyVGrid(columns: columns, spacing: 16) {
                    ForEach(model.items) { item in
                        NavigationLink(value: Route.item(item.id)) {
                            ItemCard(item: item, imageURL: model.api.imageURL(item.coverPath))
                        }
                        .buttonStyle(.plain)
                        .task { await model.loadMoreIfNeeded(after: item) }
                    }
                }
                .padding(.horizontal)
                if model.isLoading {
                    ProgressView().padding()
                }
            }
        }
        .navigationTitle("Entdecken")
        .searchable(text: $model.search, prompt: "Was suchst du?")
        .onSubmit(of: .search) { Task { await model.reload() } }
        .onChange(of: model.search) { _, value in
            if value.isEmpty { Task { await model.reload() } }
        }
        .refreshable {
            await model.reload()
            await session.refreshBadges()
        }
        .toolbar {
            ToolbarItem(placement: .topBarTrailing) {
                NavigationLink(value: Route.activity) {
                    Image(systemName: session.unreadNotifications > 0 ? "bell.badge.fill" : "bell")
                        .symbolRenderingMode(.multicolor)
                        .accessibilityLabel("Aktivität")
                }
            }
        }
        .task { await model.reload() }
        .errorAlert($model.error)
    }

    private var categoryChips: some View {
        ScrollView(.horizontal, showsIndicators: false) {
            HStack(spacing: 8) {
                chip(label: "Alle", symbol: "square.grid.2x2", selected: model.category == nil) {
                    model.category = nil
                }
                ForEach(Category.all) { category in
                    chip(label: category.label, symbol: category.symbol, selected: model.category == category.id) {
                        model.category = model.category == category.id ? nil : category.id
                    }
                }
            }
            .padding(.horizontal)
            .padding(.vertical, 8)
        }
    }

    private func chip(label: String, symbol: String, selected: Bool, action: @escaping () -> Void) -> some View {
        Button {
            action()
            Task { await model.reload() }
        } label: {
            Label(label, systemImage: symbol)
                .font(.subheadline)
                .padding(.horizontal, 12)
                .padding(.vertical, 7)
                .background(selected ? Color.swappyAccent : Color(.secondarySystemBackground), in: Capsule())
                .foregroundStyle(selected ? Color.white : Color.primary)
        }
        .buttonStyle(.plain)
    }
}
