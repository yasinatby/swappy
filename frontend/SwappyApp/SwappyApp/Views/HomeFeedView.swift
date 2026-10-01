import SwiftUI

struct HomeFeedView: View {
    @EnvironmentObject var api: APIClient
    @State private var searchText = ""
    @State private var reverseSearchText = ""
    @State private var selectedCategory: String? = nil
    @State private var items: [Item] = []
    @State private var categories: [CategoryInfo] = []
    @State private var isLoading = false
    @State private var selectedItem: Item? = nil

    let defaultCategories = [
        "Gaming", "Elektronik", "Fahrzeuge", "Kleidung",
        "Haushalt", "Hobby", "Bücher", "Uhren"
    ]

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(spacing: 20) {
                    // Header Banner
                    headerSection

                    // Search & Reverse Search Bars
                    searchSection

                    // Category Chips
                    categoriesSection

                    // Items Grid
                    itemsSection
                }
                .padding(.vertical)
            }
            .background(Color(.systemGroupedBackground))
            .navigationTitle("Swappy Marktplatz")
            .refreshable {
                await loadData()
            }
            .task {
                await loadData()
            }
            .sheet(item: $selectedItem) { item in
                ItemDetailView(item: item)
            }
        }
    }

    private var headerSection: some View {
        VStack(spacing: 8) {
            HStack {
                VStack(alignment: .leading) {
                    Text("🔄 Tauschen statt kaufen")
                        .font(.title2.bold())
                    Text("Finde perfekte Tauschpartner in deiner Nähe")
                        .font(.subheadline)
                        .foregroundStyle(.secondary)
                }
                Spacer()
            }
            .padding(.horizontal)
        }
    }

    private var searchSection: some View {
        VStack(spacing: 10) {
            // General Item Search
            HStack {
                Image(systemName: "magnifyingglass")
                    .foregroundColor(.secondary)
                TextField("Was möchtest du tauschen?", text: $searchText)
                    .onSubmit {
                        Task { await searchItems() }
                    }
                if !searchText.isEmpty {
                    Button {
                        searchText = ""
                        Task { await searchItems() }
                    } label: {
                        Image(systemName: "xmark.circle.fill")
                            .foregroundColor(.secondary)
                    }
                }
            }
            .padding(12)
            .background(Color(.secondarySystemGroupedBackground))
            .clipShape(RoundedRectangle(cornerRadius: 12))
            .padding(.horizontal)

            // "Ich suche ..." Reverse Trade Interest Search
            HStack {
                Image(systemName: "arrow.triangle.swap")
                    .foregroundColor(.blue)
                TextField("„Ich suche...“ (z. B. Nintendo Switch)", text: $reverseSearchText)
                    .onSubmit {
                        Task { await searchItems() }
                    }
                if !reverseSearchText.isEmpty {
                    Button {
                        reverseSearchText = ""
                        Task { await searchItems() }
                    } label: {
                        Image(systemName: "xmark.circle.fill")
                            .foregroundColor(.secondary)
                    }
                }
            }
            .padding(12)
            .background(Color(.secondarySystemGroupedBackground))
            .clipShape(RoundedRectangle(cornerRadius: 12))
            .padding(.horizontal)
        }
    }

    private var categoriesSection: some View {
        ScrollView(.horizontal, showsIndicators: false) {
            HStack(spacing: 8) {
                CategoryChip(
                    title: "Alle",
                    isSelected: selectedCategory == nil,
                    action: {
                        selectedCategory = nil
                        Task { await searchItems() }
                    }
                )

                ForEach(defaultCategories, id: \.self) { cat in
                    CategoryChip(
                        title: cat,
                        isSelected: selectedCategory == cat,
                        action: {
                            selectedCategory = (selectedCategory == cat) ? nil : cat
                            Task { await searchItems() }
                        }
                    )
                }
            }
            .padding(.horizontal)
        }
    }

    private var itemsSection: some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack {
                Text("Aktuelle Tauschangebote")
                    .font(.headline)
                Spacer()
                Text("\(items.count) Angebote")
                    .font(.caption)
                    .foregroundColor(.secondary)
            }
            .padding(.horizontal)

            if isLoading {
                ProgressView()
                    .frame(maxWidth: .infinity, minHeight: 150)
            } else if items.isEmpty {
                VStack(spacing: 8) {
                    Image(systemName: "shippingbox")
                        .font(.largeTitle)
                        .foregroundColor(.secondary)
                    Text("Keine Angebote gefunden")
                        .font(.subheadline)
                        .foregroundColor(.secondary)
                }
                .frame(maxWidth: .infinity, minHeight: 150)
            } else {
                LazyVGrid(columns: [GridItem(.flexible()), GridItem(.flexible())], spacing: 16) {
                    ForEach(items) { item in
                        ItemCard(item: item)
                            .onTapGesture {
                                selectedItem = item
                            }
                    }
                }
                .padding(.horizontal)
            }
        }
    }

    private func loadData() async {
        isLoading = true
        defer { isLoading = false }
        do {
            let feed = try await api.fetchHomeFeed()
            self.categories = feed.categories
            if !feed.recentItems.isEmpty {
                self.items = feed.recentItems
            } else {
                self.items = try await api.fetchItems()
            }
        } catch {
            print("Failed to load feed: \(error)")
            self.items = (try? await api.fetchItems()) ?? []
        }
    }

    private func searchItems() async {
        isLoading = true
        defer { isLoading = false }
        do {
            self.items = try await api.fetchItems(
                query: searchText.isEmpty ? nil : searchText,
                category: selectedCategory,
                tradeInterest: reverseSearchText.isEmpty ? nil : reverseSearchText
            )
        } catch {
            print("Search failed: \(error)")
        }
    }
}

struct CategoryChip: View {
    let title: String
    let isSelected: Bool
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            Text(title)
                .font(.subheadline.weight(isSelected ? .bold : .regular))
                .padding(.horizontal, 14)
                .padding(.vertical, 8)
                .background(isSelected ? Color.blue : Color(.secondarySystemGroupedBackground))
                .foregroundColor(isSelected ? .white : .primary)
                .clipShape(Capsule())
        }
    }
}

struct ItemCard: View {
    let item: Item

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            // Image / Placeholder
            ZStack(alignment: .topTrailing) {
                RoundedRectangle(cornerRadius: 12)
                    .fill(Color.blue.opacity(0.1))
                    .frame(height: 120)
                    .overlay(
                        categoryIcon(for: item.category)
                            .font(.system(size: 40))
                            .foregroundColor(.blue.opacity(0.8))
                    )

                // Status Badge
                HStack(spacing: 4) {
                    Circle()
                        .fill(item.status == "available" ? Color.green : Color.orange)
                        .frame(width: 8, height: 8)
                    Text(item.status == "available" ? "Verfügbar" : "Angefragt")
                        .font(.caption2.bold())
                }
                .padding(6)
                .background(.ultraThinMaterial)
                .clipShape(Capsule())
                .padding(6)
            }

            VStack(alignment: .leading, spacing: 4) {
                Text(item.title)
                    .font(.subheadline.bold())
                    .lineLimit(1)

                Text("Wert: ca. \(Int(item.estimatedValue)) €")
                    .font(.caption.bold())
                    .foregroundColor(.blue)

                if !item.tradeInterests.isEmpty {
                    Text("Sucht: \(item.tradeInterests.joined(separator: ", "))")
                        .font(.caption2)
                        .foregroundColor(.secondary)
                        .lineLimit(1)
                }

                if let city = item.city, !city.isEmpty {
                    Label(city, systemImage: "location")
                        .font(.caption2)
                        .foregroundColor(.secondary)
                }
            }
            .padding(.horizontal, 4)
            .padding(.bottom, 6)
        }
        .padding(6)
        .background(Color(.secondarySystemGroupedBackground))
        .clipShape(RoundedRectangle(cornerRadius: 16))
        .shadow(color: .black.opacity(0.04), radius: 4, y: 2)
    }

    private func categoryIcon(for cat: String) -> Image {
        switch cat {
        case "Gaming": return Image(systemName: "gamecontroller.fill")
        case "Elektronik": return Image(systemName: "iphone.gen3")
        case "Fahrzeuge": return Image(systemName: "bicycle")
        case "Kleidung": return Image(systemName: "tshirt.fill")
        case "Bücher": return Image(systemName: "book.fill")
        case "Uhren": return Image(systemName: "applewatch")
        default: return Image(systemName: "gift.fill")
        }
    }
}
