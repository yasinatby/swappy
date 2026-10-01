import SwiftUI

struct SwapsView: View {
    @EnvironmentObject var api: APIClient
    @State private var swaps: [Swap] = []
    @State private var selectedFilter: String = "Alle"
    @State private var isLoading = false
    @State private var selectedSwap: Swap? = nil

    let filters = ["Alle", "Offen", "Vereinbart", "Abgeschlossen"]

    var filteredSwaps: [Swap] {
        switch selectedFilter {
        case "Offen":
            return swaps.filter { $0.status == "pending" || $0.status == "countered" }
        case "Vereinbart":
            return swaps.filter { $0.status == "agreed" }
        case "Abgeschlossen":
            return swaps.filter { $0.status == "completed" }
        default:
            return swaps
        }
    }

    var body: some View {
        NavigationStack {
            VStack(spacing: 0) {
                // Filter Picker
                Picker("Status", selection: $selectedFilter) {
                    ForEach(filters, id: \.self) { f in
                        Text(f).tag(f)
                    }
                }
                .pickerStyle(.segmented)
                .padding()

                if isLoading {
                    Spacer()
                    ProgressView()
                    Spacer()
                } else if filteredSwaps.isEmpty {
                    Spacer()
                    VStack(spacing: 10) {
                        Image(systemName: "arrow.triangle.swap")
                            .font(.system(size: 44))
                            .foregroundColor(.secondary)
                        Text("Keine Tauschgeschäfte in dieser Ansicht")
                            .font(.subheadline)
                            .foregroundColor(.secondary)
                    }
                    Spacer()
                } else {
                    List {
                        ForEach(filteredSwaps) { swap in
                            SwapRowView(swap: swap)
                                .contentShape(Rectangle())
                                .onTapGesture {
                                    selectedSwap = swap
                                }
                        }
                    }
                    .listStyle(.insetGrouped)
                }
            }
            .background(Color(.systemGroupedBackground))
            .navigationTitle("Meine Täusche")
            .refreshable {
                await loadSwaps()
            }
            .task {
                await loadSwaps()
            }
            .sheet(item: $selectedSwap) { swap in
                SwapDetailView(swap: swap, onUpdated: {
                    Task { await loadSwaps() }
                })
            }
        }
    }

    private func loadSwaps() async {
        guard api.isAuthenticated else { return }
        isLoading = true
        defer { isLoading = false }
        do {
            self.swaps = try await api.fetchSwaps()
        } catch {
            print("Failed to load swaps: \(error)")
        }
    }
}

struct SwapRowView: View {
    let swap: Swap

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            HStack {
                statusBadge(swap.status)
                Spacer()
                if swap.cashAdjustment != 0 {
                    Text(swap.cashAdjustment > 0 ? "+\(Int(swap.cashAdjustment)) €" : "\(Int(swap.cashAdjustment)) €")
                        .font(.caption.bold())
                        .foregroundColor(.blue)
                }
            }

            HStack(spacing: 12) {
                VStack(alignment: .leading, spacing: 2) {
                    Text("Zielobjekt:")
                        .font(.caption2)
                        .foregroundColor(.secondary)
                    Text(swap.targetItem?.title ?? "Zielobjekt")
                        .font(.subheadline.bold())
                }

                Image(systemName: "arrow.right")
                    .foregroundColor(.secondary)
                    .font(.caption)

                VStack(alignment: .leading, spacing: 2) {
                    Text("Angeboten:")
                        .font(.caption2)
                        .foregroundColor(.secondary)
                    Text(swap.offeredItems?.first?.title ?? "Angebotenes Objekt")
                        .font(.subheadline.bold())
                }
            }

            if let partner = (swap.initiator?.id == swap.initiatorId ? swap.recipient : swap.initiator) {
                HStack {
                    Image(systemName: "person.circle")
                        .foregroundColor(.secondary)
                    Text("Partner: @\(partner.username)")
                        .font(.caption)
                        .foregroundColor(.secondary)
                }
            }
        }
        .padding(.vertical, 4)
    }

    @ViewBuilder
    private func statusBadge(_ status: String) -> some View {
        switch status {
        case "pending":
            Text("🟡 Angebot offen")
                .font(.caption.bold())
                .foregroundColor(.orange)
        case "countered":
            Text("⚖️ Gegenangebot")
                .font(.caption.bold())
                .foregroundColor(.purple)
        case "agreed":
            Text("🔄 Tausch vereinbart")
                .font(.caption.bold())
                .foregroundColor(.blue)
        case "completed":
            Text("🤝 Abgeschlossen")
                .font(.caption.bold())
                .foregroundColor(.green)
        case "rejected":
            Text("❌ Abgelehnt")
                .font(.caption.bold())
                .foregroundColor(.red)
        default:
            Text(status.capitalized)
                .font(.caption.bold())
        }
    }
}
